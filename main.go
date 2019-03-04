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
	"sync"

	//"os"
	"strconv"
	"strings"
)

type Config struct {
	Database string
	Token    string
	Channel  string
	Server   string
	Port     string
	Rcon     string

	EUServers []string
	NAServers []string
	AUServers []string
}

var filename string = ""

var config Config

func main() {
	exe, _ := os.Executable()

	exePath := filepath.Dir(exe)

	filename = exePath + "/config.json"

	var wg sync.WaitGroup

	err := gonfig.GetConf(filename, &config)
	if err != nil {
		panic(err.Error())
	}

	// launch a bsion bot for each official kag server
	for i := 0; i < len(config.EUServers); i++ {
		wg.Add(1)
		go bsion(&wg, config.EUServers[i])
	}

	for i := 0; i < len(config.NAServers); i++ {
		wg.Add(1)
		go bsion(&wg, config.NAServers[i])
	}

	for i := 0; i < len(config.AUServers); i++ {
		wg.Add(1)
		go bsion(&wg, config.AUServers[i])
	}

	wg.Wait()
}

func bsion(wg *sync.WaitGroup, serverIp string) {
	conn, discord, db := connect(serverIp)

	if conn != nil && discord != nil && db != nil {
		defer conn.Close()
		defer discord.Close()
		defer db.Close()

		defer wg.Done()

		// authenticate to server as rcon
		_, err := conn.Write([]byte(config.Rcon + "\n"))
		if err != nil {
			log.Println(err)
		}

		// open a websocket connection to Discord and begin listening.
		err = discord.Open()
		if err != nil {
			log.Println("couldn't open discord,", err)
			return
		}

		listen(conn, discord, db)
	}
}

func connect(serverIp string) (net.Conn, *discordgo.Session, *sql.DB) {
	// start tcp connection to kag server
	conn, err := net.Dial("tcp", serverIp)
	if err != nil {
		log.Println("Error can't connect...", err)
		return nil, nil, nil
	}

	// start connection to discord api
	discord, err := discordgo.New("Bot " + config.Token)
	if err != nil {
		panic(err)
	}

	// start connection to localhost database
	db, err := sql.Open("mysql", config.Database)
	if err != nil {
		panic(err)
	}

	return conn, discord, db
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

func listen(conn net.Conn, session *discordgo.Session, db *sql.DB) {
	reader := bufio.NewReader(conn)

	for {
		message, err := reader.ReadString('\n')
		if err != nil {
			log.Println("cant read message,", err)
			break
		}

		fmt.Println([]byte(message))
		fmt.Println(message)

		if strings.Contains(message, "*REPORT") {

			tokens := strings.Split(message, " ")
			reporter, baddie, reportcount := tokens[2], tokens[3], tokens[4]

			reportcount = strings.TrimSpace(reportcount)

			reportCountInt, err := strconv.Atoi(strings.TrimSpace(reportcount))
			if err != nil {
				log.Println("reportCount isn't an int,", err)
				break
			}

			fmt.Println("got message")

			if reportCountInt >= 2 {
				_, err := session.ChannelMessageSend(config.Channel, "@here Player " + reporter + " has reported player " + baddie + " for a total of " + reportcount + " reports.")
				if err != nil {
					log.Println("cant send message,", err)
					break
				}
			} else {
				_, err := session.ChannelMessageSend(config.Channel, "Player " + reporter + " has reported player " + baddie + " for a total of " + reportcount + " report.")
				if err != nil {
					log.Println("cant send message,", err)
					break
				}
			}

			dbwrite(db, baddie, reportcount)
		}
	}
}
