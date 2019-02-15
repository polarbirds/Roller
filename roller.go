package main

import (
	"github.com/bwmarrin/discordgo"
)

var (
	commandPrefix string
	botId         string
	botToken      string
)

func main() {
	discord, err := discordgo.New("Bot " + botToken)

}
