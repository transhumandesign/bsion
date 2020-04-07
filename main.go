package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"github.com/bwmarrin/discordgo"
	_ "github.com/go-sql-driver/mysql"
	"github.com/tkanos/gonfig"
	"log"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type config struct {
	Database string
	Token    string
	Channel  string
	Server   string
	Port     string
	Rcon     string

	EUServers []string
	NAServers []string
	AUServers []string
	Others []string
}

var filename string = ""
var configFile config

var connectedToKag = false

func main() {
	exe, _ := os.Executable()
	exePath := filepath.Dir(exe)
	filename = exePath + "/config.json"
	println(filename)

	err := gonfig.GetConf(filename, &configFile)
	if err != nil {
		panic(err.Error())
	}

	var wg sync.WaitGroup

	// launch a bsion bot for each official kag server
	for i := 0; i < len(configFile.EUServers); i++ {
		// wg.Add(1)
		// go bsion(&wg, configFile.EUServers[i], configFile.Rcon)
	}

	for i := 0; i < len(configFile.NAServers); i++ {
		// wg.Add(1)
		// go bsion(&wg, configFile.NAServers[i], configFile.Rcon)
	}

	for i := 0; i < len(configFile.AUServers); i++ {
		// wg.Add(1)
		// go bsion(&wg, configFile.AUServers[i], configFile.Rcon)
	}

	for i := 0; i < len(configFile.Others); i++ {
		wg.Add(1)
		go bsion(&wg, configFile.Others[i], "poop")
	}

	wg.Wait()
}

func bsion(wg *sync.WaitGroup, serverIP string, pw string) {
	conn := connectToKag(serverIP, pw)
	discord := connectToDiscord()
	db := connectToSQL()

	if conn != nil && discord != nil && db != nil {
		defer conn.Close()
		defer discord.Close()
		defer db.Close()

		defer wg.Done()

		listen(conn, discord, db, pw)
	}
}

func listen(conn net.Conn, session *discordgo.Session, db *sql.DB, pw string) {
	reader := bufio.NewReader(conn)

	for {
		message, err := reader.ReadString('\n')
		if err != nil {
			log.Println("error reading message. ", err)
			connectedToKag = false
			conn = connectToKag(conn.RemoteAddr().String(), pw)

			reader.Reset(reader)
			reader = bufio.NewReader(conn)

			continue
		}

		//fmt.Println([]byte(message))
		fmt.Println(message)

		if strings.Contains(message, "*REPORT") {

			regex := regexp.MustCompile("\\*REPORT \\*PLAYER=\\\"(.*?)\\\" \\*BADDIE=\\\"(.*?)\\\" \\*COUNT=\\\"(\\d*?)\\\" \\*SERVERNAME=\\\"(.*?)\\\" \\*SERVERIP=\\\"(.*?)\\\" \\*REASON=\\\"(.*?)\\\"")

			tokens := regex.FindStringSubmatch(message)
			if err != nil {
				log.Println("can't find substring,", err)
				break
			}
			fmt.Println(tokens[1:])

			//tokens := strings.Split(message, " ")
			player, baddie, reportcount, servername, serverip, reason := tokens[1], tokens[2], tokens[3], tokens[4], tokens[5], tokens[6]

			serverlink := "<kag://" + serverip + "/>"

			reportcount = strings.TrimSpace(reportcount)
			reason = strings.TrimSpace(reason);

			reportCountInt, err := strconv.Atoi(strings.TrimSpace(reportcount))
			if err != nil {
				log.Println("reportcount isn't an int,", err)
				break
			}

			fmt.Println("got message")

			if reportCountInt >= 2 {
				_, err := session.ChannelMessageSend(configFile.Channel,
					"@here " + baddie + " has been reported by " + player + " for a total of " + reportcount + " reports\n" +
						"Reason: " + "\"" + reason + "\"\n" +
						"Server: " + servername + "\n" +
						"Address: " + serverlink)

				if err != nil {
					log.Println("cant send message,", err)
					break
				}
			} else {
				_, err := session.ChannelMessageSend(configFile.Channel,
					"@here " + baddie + " has been reported by " + player + " for a total of " + reportcount + " report\n" +
						"Reason: " + "\"" + reason + "\"\n" +
						"Server: " + servername + "\n" +
						"Address: " + serverlink)

				if err != nil {
					log.Println("cant send message,", err)
					break
				}
			}

			dbwrite(db, baddie, reportcount)
		}
	}
}

func dbwrite(db *sql.DB, playerName, reportcount string) {
	_, err := db.Exec("insert into `reports` (`player_name`, `report_count`, `last_date`) values (?, ?, NOW()) on duplicate key update `report_count` = `report_count` + 1, `last_date` = NOW()", playerName, reportcount)

	// if there is an error inserting, handle it
	if err != nil {
		log.Println("cant write message to db,", err)
	}

	//defer insert.Close()

	log.Println("wrote to db")
}

func connectToKag(serverIP string, pw string) net.Conn {
	var conn net.Conn
	var err error

	if connectedToKag == false {
		// start tcp connection to kag server
		for connectedToKag != true {
			log.Println("connecting to KAG server...")
			conn, err = net.Dial("tcp", serverIP)

			if err != nil {
				log.Println("couln't connect to kag server. ", err)
			} else {
				log.Println("connected succesfully")
				connectedToKag = true
				break
			}

			time.Sleep(30 * 1000 * time.Millisecond)
		}

		// authenticate to server as rcon
		_, err = conn.Write([]byte(pw + "\n"))
		if err != nil {
			log.Println("couldn't login as rcon, ", err)
		}
	}

	return conn
}

func connectToSQL() *sql.DB {
	db, err := sql.Open("mysql", configFile.Database)
	if err != nil {
		panic(err)
	}

	return db
}

func connectToDiscord() *discordgo.Session {
	discord, err := discordgo.New("Bot " + configFile.Token)
	if err != nil {
		panic(err)
	}

	// open a websocket connection to Discord and begin listening.
	err = discord.Open()
	if err != nil {
		panic(err)
		return nil
	}

	return discord
}
