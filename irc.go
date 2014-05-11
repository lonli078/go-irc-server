package main

import (
    "fmt";
    "net";
    "log";
    "container/list";
    "flag";
    "strings";
    "errors"
)

var debug = flag.Bool("d", false, "set the debug modus( print informations )")
var realm = flag.String("realm", "192.168.0.103", "set the realm of server")

func Log(v ...string) {
    if *debug == true {
        log.Printf("SERVER: %s", fmt.Sprint(v));
    }
}

type ClientChat struct {
    Name string
    IN chan string
    OUT chan string
    Con net.Conn
    Quit chan bool
    ListChain *list.List
    ListChannel *list.List;
}

type ChannelChat struct {
    Name string
	Topic string
    UsersList *list.List
}

func (c ClientChat) Read(buf []byte) bool{
    _, err := c.Con.Read(buf);
    if err!=nil {
        c.Close();
        return false;
    }
    return true;
}

func (c *ClientChat) Close() {
    c.Quit<-true;
    c.Con.Close();
    c.deleteFromList();
    Log("clientsender(): want to quit", c.Name);
    Log(fmt.Sprintf("CLIENTINLIST: %d", c.ListChain.Len()))
}

func (c *ClientChat) Equal(cl *ClientChat) bool {
    if c.Con == cl.Con {
        return true;
    }
    return false;
}

func (c *ClientChat) deleteFromList() {
    for e := c.ListChain.Front(); e != nil; e = e.Next() {
        client := e.Value.(ClientChat);
        if c.Equal(&client) {
            Log("deleteFromList(): ", c.Name);
            c.ListChain.Remove(e);
        }
    }
    for e := c.ListChannel.Front(); e != nil; e = e.Next() {
        ch := e.Value.(ChannelChat);
        ch.removeuser(c)
    }
}

func (c *ClientChat) send_message(prefix string, command string, params ...string) {
	var msg string
	if len(prefix) != 0 {
		msg = fmt.Sprintf(":%s %s", prefix, command)
	} else {
		msg = fmt.Sprintf(":%s %s", *realm, command)
	}
	for _, v := range params {
		msg = msg + " " + v
	}
	c.IN <- msg
}

func (c *ClientChat) get_channel(name string) *ChannelChat {
	//Log("ClientChat.get_channel()", "List chan len->", fmt.Sprintf(":%d", c.ListChannel.Len() ))
	for e := c.ListChannel.Front(); e != nil; e = e.Next() {
		ch := e.Value.(ChannelChat);
		if ch.Name == name {
			//Log("ClientChat.get_channel()", "channel found!", ch.Name)
			return &ch
		}
	}
	Log("ClientChat.get_channel()", "create new channel->", name)
	ch := &ChannelChat{name, "", list.New()}
	c.ListChannel.PushBack(*ch)
	return ch
}

func (ch *ChannelChat) userjoin(user *ClientChat) {
	ch.UsersList.PushBack(*user);
	Log("ChannelChat.userjoin()", fmt.Sprintf("Chan: %s In chan: %d", ch.Name, ch.UsersList.Len()))
}
func (ch *ChannelChat) removeuser(user *ClientChat) {
	for cl := ch.UsersList.Front(); cl != nil; cl = cl.Next() {
		client := cl.Value.(ClientChat);
		if user.Equal(&client) {
			Log("deleteFromChList(): ", user.Name);
			ch.UsersList.Remove(cl);
			ch.updateuserlist()
		}
	}
}

func (ch *ChannelChat) updateuserlist() {
	for cl := ch.UsersList.Front(); cl != nil; cl = cl.Next() {
		client := cl.Value.(ClientChat);
		send_client_list(&client, ch)
	}
}

func (ch *ChannelChat) sendmsg(user *ClientChat, msg string) {
	Log("ChannelChat.sendmsg()", fmt.Sprintf("Chan: %s In chan: %d", ch.Name, ch.UsersList.Len()))
	for e := ch.UsersList.Front(); e != nil; e = e.Next() {
		client := e.Value.(ClientChat);
		if !client.Equal(user) {
			prefix := fmt.Sprintf("%s!%s@%s", user.Name, user.Name, user.Con.RemoteAddr())
			Log("ChannelChat.sendmsg() From:", user.Name, "TO:", client.Name)
			client.send_message(prefix, "PRIVMSG", ch.Name, msg)
		}
	}
}

func (ch *ChannelChat) getuser(nickname string) (ClientChat, error) {
	for e := ch.UsersList.Front(); e != nil; e = e.Next() {
		client := e.Value.(ClientChat)
		if client.Name == nickname {
			return client, nil
		}
	}
	return ClientChat{}, errors.New("New error")
}

func (ch *ChannelChat) count() int {
	return ch.UsersList.Len()
}

func irc_NICK(client *ClientChat, params []string) {
	/*
        <nickname>
        ERR_NONICKNAMEGIVEN +
        ERR_ERRONEUSNICKNAME
        ERR_NICKNAMEINUSE
        ERR_NICKCOLLISION
        ERR_UNAVAILRESOURCE
        ERR_RESTRICTED
	*/
	if len(params) == 0 {
		client.send_message("", stn["ERR_NONICKNAMEGIVEN"], client.Name, ":No nickname given.")
		return
	}
	client.Name = params[0]
	Log("irc_NICK: Set nickname->", client.Name)
}
func irc_USER(client *ClientChat, params []string) {
	/*
        <user> <mode> <unused> <realname>
        ERR_NEEDMOREPARAMS +
        ERR_ALREADYREGISTRED
    */
    if len(params) < 4 {
		client.send_message("", stn["ERR_NEEDMOREPARAMS"], client.Name, ":Need more params.")
	}
	send_welcome(client)
	send_motd(client)
}

func send_client_list(client *ClientChat, ch *ChannelChat) {
	namereply := ""
	for e := ch.UsersList.Front(); e != nil; e = e.Next() {
		cl := e.Value.(ClientChat);
		namereply += cl.Name + " "
	}
	client.send_message("", stn["RPL_NAMREPLY"], client.Name, "=", ch.Name, fmt.Sprintf(":%s", namereply))
	client.send_message("", stn["RPL_ENDOFNAMES"], client.Name, ch.Name, ":End of /NAMES list.")
}

func irc_JOIN(client *ClientChat, params []string) {
	/*
        <channel> *(", " <channel>) [ <key> *(", " <key>) ])

        ERR_NEEDMOREPARAMS
        ERR_BANNEDFROMCHAN
        ERR_INVITEONLYCHAN
        ERR_BADCHANNELKEY
        ERR_CHANNELISFULL
        ERR_BADCHANMASK
        ERR_NOSUCHCHANNEL
        ERR_TOOMANYCHANNELS
        ERR_TOOMANYTARGETS
        ERR_UNAVAILRESOURCE
        RPL_TOPIC +
	*/
	name := params[0]
	if !strings.HasPrefix(name, "#") {
		name = "#" + name
	}
	ch := client.get_channel(name)
	ch.userjoin(client)

	prefix := fmt.Sprintf("%s!~%s@%s", client.Name, client.Name, *realm)
	client.send_message(prefix, "JOIN", fmt.Sprintf(":%s", ch.Name))
	send_client_list(client, ch)
	client.send_message("", stn["RPL_TOPIC"], client.Name, ch.Name, fmt.Sprintf(":%s", ch.Topic))
	for e := ch.UsersList.Front(); e != nil; e = e.Next() {
		cl := e.Value.(ClientChat);
		send_client_list(&cl, ch)
	}
}

func irc_PRIVMSG(client *ClientChat, params []string) {
	/*
        <msgtarget> <text to be sent>
        ERR_NORECIPIENT
        ERR_NOTEXTTOSEND
        ERR_CANNOTSENDTOCHAN
        ERR_NOTOPLEVEL
        ERR_WILDTOPLEVEL
        ERR_TOOMANYTARGETS
        ERR_NOSUCHNICK
        RPL_AWAY
	*/
	recipient := params[0]
	msg := strings.Join(params[1:], "")
	if strings.HasPrefix(recipient, "#") {
		ch := client.get_channel(recipient)
        ch.sendmsg(client, msg)
        return
	} else {
		for e := client.ListChannel.Front(); e != nil; e = e.Next() {
			ch := e.Value.(ChannelChat);
			rec, err := ch.getuser(recipient)
			if err == nil {
				prefix := fmt.Sprintf("%s!~%s@%s", client.Name, client.Name, *realm)
				rec.send_message(prefix, "PRIVMSG", rec.Name, msg)
			}
		}
	}
}

func irc_PING(client *ClientChat, params []string) {
	client.send_message("", "PONG")
}

func irc_QUIT(client *ClientChat, params []string) {
	/*
         [ <Quit Message> ]
	*/
	client.send_message("", "QUIT", fmt.Sprintf(":Quit: %s", params[0]))
	client.Close()
}

func irc_WHO(client *ClientChat, params []string) {
	/*
        [ <mask> [ «o» ] ]
        ERR_NOSUCHSERVER
        RPL_WHOREPLY +
        RPL_ENDOFWHO +
	*/
	ch := client.get_channel(params[0])
	for e := ch.UsersList.Front(); e != nil; e = e.Next() {
		cl := e.Value.(ClientChat);
		client.send_message("", stn["RPL_WHOREPLY"], client.Name, ch.Name, cl.Name)
	}
	client.send_message("", stn["RPL_ENDOFWHO"], client.Name, ch.Name, ":End of /WHO list.")
}

func irc_PART(client *ClientChat, params []string) {
	/*
        <channel> *(", « <channel>) [ <Part Message> ]
        ERR_NEEDMOREPARAMS
        ERR_NOSUCHCHANNEL
        ERR_NOTONCHANNEL
	*/
	ch := client.get_channel(params[0])
	ch.removeuser(client)
	prefix := fmt.Sprintf("%s!~%s@%s", client.Name, client.Name, *realm)
	client.send_message(prefix, "PART", ch.Name, ":good bye")
}

func irc_LIST(client *ClientChat, params []string) {
	/*
        [ <channel> *(", " <channel>) [ <target> ] ]
        ERR_TOOMANYMATCHES -
        ERR_NOSUCHSERVER -
        RPL_LIST +
        RPL_LISTEND +
	*/
	client.send_message("", stn["RPL_LISTSTART"], client.Name, "Channel :Users  Name")
	for e := client.ListChannel.Front(); e != nil; e = e.Next() {
		ch := e.Value.(ChannelChat);
		client.send_message("", stn["RPL_LIST"], client.Name, ch.Name, fmt.Sprintf("%d", ch.count()), ":[+nt] " + ch.Topic)
	}
	client.send_message("", stn["RPL_LISTEND"], client.Name, ":END of /LIST")
}

func send_welcome(client *ClientChat) {
	client.send_message("", "NOTICE", "AUTH :*** You connected on 6667 port")
	client.send_message("", stn["RPL_WELCOME"], client.Name, ":Welcome to Dev Team IRC")
	client.send_message("", stn["RPL_YOURHOST"], client.Name, ":Your host is 127.0.0.1, running version 0.01")
	client.send_message("", stn["RPL_CREATED"], client.Name, ":This server was created now")
	client.send_message("", stn["RPL_MYINFO"], client.Name, "www.site.com", "goircserver", "iowghraAsORTVSxNCWqBzvdHtGpZI", "lvhopsmntikrRcaqOALQbSeIKVfMCuzNTGjP")
}

func send_motd(client *ClientChat) {
	client.send_message("", stn["RPL_MOTDSTART"], client.Name, ":- Message of the Day - ")
	client.send_message("", stn["RPL_MOTD"], client.Name, ":Hello mazafaca")
	client.send_message("", stn["RPL_ENDOFMOTD"], client.Name, ":End of /MOTD command.")
}

func clientreceiver(client *ClientChat) {
    buf := make([]byte, 2048);

    Log("clientreceiver(): start for: ", client.Name);
    for client.Read(buf) {
        for _, msg := range strings.Split(string(buf), "\r\n") {
			//Black magic TODO WTF
			if []byte(msg)[0] != 0 {
				Log("GET:", msg)
				command := strings.ToUpper(strings.Split(msg, " ")[0])
				params := strings.Split(msg, " ")[1:]
				cm, found := commands_list[command]
				if found {
					cm(client, params)
				} else {
					Log("MSG:", "unknown message >>>", msg)
				}
			}
        }
        for i:=0; i<2048;i++ {
            buf[i]=0x00;
        }
    }
    Log("clientreceiver(): stop for: ", client.Name);
}

func clientsender(client *ClientChat) {
    Log("clientsender(): start for: ", client.Name);
    for {
        Log("clientsender(): wait for input to send");
        select {
            case buf := <- client.IN:
				Log("SEND:", buf)
                client.Con.Write([]byte(buf + "\r\n"));
            case <-client.Quit:
                break;
        }
    }
    Log("clientsender(): stop for: ", client.Name);
}

func clientHandling(con net.Conn, userlst *list.List, channellst *list.List) {
    newclient := &ClientChat{"", make(chan string), make(chan string), con, make(chan bool), userlst, channellst};
    go clientsender(newclient);
    go clientreceiver(newclient);
    userlst.PushBack(*newclient);
}

func main() {
    flag.Parse();
    Log("main: start IRC server...");
    clientlist := list.New();
    channellist := list.New();
    netlisten, _ := net.Listen("tcp", "0.0.0.0:6667");
    defer netlisten.Close();
    for {
        Log("main: wait for client ...");
        conn, _ := netlisten.Accept();
        go clientHandling(conn, clientlist, channellist);
    }
}


var commands_list = map[string]func(*ClientChat, []string) {
	/*
	* PASS
	* TOPIC
	* MODT
	*/
	"NICK": irc_NICK,
	"USER": irc_USER,
	"JOIN": irc_JOIN,
	"PRIVMSG": irc_PRIVMSG,
	"PING": irc_PING,
	"QUIT": irc_QUIT,
	"WHO": irc_WHO,
	"PART": irc_PART,
	"LIST": irc_LIST,
}

var stn = map[string]string {
	"RPL_WELCOME": "001",
    "RPL_YOURHOST": "002",
    "RPL_CREATED": "003",
    "RPL_MYINFO": "004",
    "RPL_ISUPPORT": "005",
    "RPL_BOUNCE": "010",
    "RPL_USERHOST": "302",
    "RPL_ISON": "303",
    "RPL_AWAY": "301",
    "RPL_UNAWAY": "305",
    "RPL_NOWAWAY": "306",
    "RPL_WHOISUSER": "311",
    "RPL_WHOISSERVER": "312",
    "RPL_WHOISOPERATOR": "313",
    "RPL_WHOISIDLE": "317",
    "RPL_ENDOFWHOIS": "318",
    "RPL_WHOISCHANNELS": "319",
    "RPL_WHOWASUSER": "314",
    "RPL_ENDOFWHOWAS": "369",
    "RPL_LISTSTART": "321",
    "RPL_LIST": "322",
    "RPL_LISTEND": "323",
    "RPL_UNIQOPIS": "325",
    "RPL_CHANNELMODEIS": "324",
    "RPL_NOTOPIC": "331",
    "RPL_TOPIC": "332",
    "RPL_INVITING": "341",
    "RPL_SUMMONING": "342",
    "RPL_INVITELIST": "346",
    "RPL_ENDOFINVITELIST": "347",
    "RPL_EXCEPTLIST": "348",
    "RPL_ENDOFEXCEPTLIST": "349",
    "RPL_VERSION": "351",
    "RPL_WHOREPLY": "352",
    "RPL_ENDOFWHO": "315",
    "RPL_NAMREPLY": "353",
    "RPL_ENDOFNAMES": "366",
    "RPL_LINKS": "364",
    "RPL_ENDOFLINKS": "365",
    "RPL_BANLIST": "367",
    "RPL_ENDOFBANLIST": "368",
    "RPL_INFO": "371",
    "RPL_ENDOFINFO": "374",
    "RPL_MOTDSTART": "375",
    "RPL_MOTD": "372",
    "RPL_ENDOFMOTD": "376",
    "RPL_YOUREOPER": "381",
    "RPL_REHASHING": "382",
    "RPL_YOURESERVICE": "383",
    "RPL_TIME": "391",
    "RPL_USERSSTART": "392",
    "RPL_USERS": "393",
    "RPL_ENDOFUSERS": "394",
    "RPL_NOUSERS": "395",
    "RPL_TRACELINK": "200",
    "RPL_TRACECONNECTING": "201",
    "RPL_TRACEHANDSHAKE": "202",
    "RPL_TRACEUNKNOWN": "203",
    "RPL_TRACEOPERATOR": "204",
    "RPL_TRACEUSER": "205",
    "RPL_TRACESERVER": "206",
    "RPL_TRACESERVICE": "207",
    "RPL_TRACENEWTYPE": "208",
    "RPL_TRACECLASS": "209",
    "RPL_TRACERECONNECT": "210",
    "RPL_TRACELOG": "261",
    "RPL_TRACEEND": "262",
    "RPL_STATSLINKINFO": "211",
    "RPL_STATSCOMMANDS": "212",
    "RPL_ENDOFSTATS": "219",
    "RPL_STATSUPTIME": "242",
    "RPL_STATSOLINE": "243",
    "RPL_UMODEIS": "221",
    "RPL_SERVLIST": "234",
    "RPL_SERVLISTEND": "235",
    "RPL_LUSERCLIENT": "251",
    "RPL_LUSEROP": "252",
    "RPL_LUSERUNKNOWN": "253",
    "RPL_LUSERCHANNELS": "254",
    "RPL_LUSERME": "255",
    "RPL_ADMINME": "256",
    "RPL_ADMINLOC": "257",
    "RPL_ADMINEMAIL": "259",
    "RPL_TRYAGAIN": "263",
    "ERR_NOSUCHNICK": "401",
    "ERR_NOSUCHSERVER": "402",
    "ERR_NOSUCHCHANNEL": "403",
    "ERR_CANNOTSENDTOCHAN": "404",
    "ERR_TOOMANYCHANNELS": "405",
    "ERR_WASNOSUCHNICK": "406",
    "ERR_TOOMANYTARGETS": "407",
    "ERR_NOSUCHSERVICE": "408",
    "ERR_NOORIGIN": "409",
    "ERR_NORECIPIENT": "411",
    "ERR_NOTEXTTOSEND": "412",
    "ERR_NOTOPLEVEL": "413",
    "ERR_WILDTOPLEVEL": "414",
    "ERR_BADMASK": "415",
    "ERR_UNKNOWNCOMMAND": "421",
    "ERR_NOMOTD": "422",
    "ERR_NOADMININFO": "423",
    "ERR_FILEERROR": "424",
    "ERR_NONICKNAMEGIVEN": "431",
    "ERR_ERRONEUSNICKNAME": "432",
    "ERR_NICKNAMEINUSE": "433",
    "ERR_NICKCOLLISION": "436",
    "ERR_UNAVAILRESOURCE": "437",
    "ERR_USERNOTINCHANNEL": "441",
    "ERR_NOTONCHANNEL": "442",
    "ERR_USERONCHANNEL": "443",
    "ERR_NOLOGIN": "444",
    "ERR_SUMMONDISABLED": "445",
    "ERR_USERSDISABLED": "446",
    "ERR_NOTREGISTERED": "451",
    "ERR_NEEDMOREPARAMS": "461",
    "ERR_ALREADYREGISTRED": "462",
    "ERR_NOPERMFORHOST": "463",
    "ERR_PASSWDMISMATCH": "464",
    "ERR_YOUREBANNEDCREEP": "465",
    "ERR_YOUWILLBEBANNED": "466",
    "ERR_KEYSET": "467",
    "ERR_CHANNELISFULL": "471",
    "ERR_UNKNOWNMODE": "472",
    "ERR_INVITEONLYCHAN": "473",
    "ERR_BANNEDFROMCHAN": "474",
    "ERR_BADCHANNELKEY": "475",
    "ERR_BADCHANMASK": "476",
    "ERR_NOCHANMODES": "477",
    "ERR_BANLISTFULL": "478",
    "ERR_NOPRIVILEGES": "481",
    "ERR_CHANOPRIVSNEEDED": "482",
    "ERR_CANTKILLSERVER": "483",
    "ERR_RESTRICTED": "484",
    "ERR_UNIQOPPRIVSNEEDED": "485",
    "ERR_NOOPERHOST": "491",
    "ERR_NOSERVICEHOST": "492",
    "ERR_UMODEUNKNOWNFLAG": "501",
    "ERR_USERSDONTMATCH": "502",
    }

