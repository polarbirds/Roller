package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/bwmarrin/discordgo"
	log "github.com/sirupsen/logrus"
)

func main() {
	var Token string
	flag.StringVar(&Token, "t", "", "Bot Token")
	flag.Parse()

	dg, err := discordgo.New("Bot " + Token)
	if err != nil {
		log.Error("error creating Discord session,", err)
		return
	}

	c := cardinal{
		s: dg,
	}

	dg.AddHandler(c.messageCreate)

	err = dg.Open()
	if err != nil {
		log.Error("error opening connection,", err)
		return
	}

	log.Info("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	dg.Close()
}

func isValidCommand(command string) bool {
	switch command {
	case
		"me",
		"!me",
		"em",
		"!em":
		return true
	}
	return false
}

type cardinal struct {
	s *discordgo.Session
}

func (c *cardinal) getGuild(msg *discordgo.Message) (g *discordgo.Guild, err error) {
	channel, err := c.s.Channel(msg.ChannelID)
	if err != nil {
		return
	}

	g, err = c.s.Guild(channel.GuildID)
	return
}

func (c *cardinal) createRole(name string, guild *discordgo.Guild, color int) (role *discordgo.Role, err error) {
	role, err = c.s.GuildRoleCreate(guild.ID)
	if err != nil {
		return
	}

	role, err = c.s.GuildRoleEdit(guild.ID, role.ID, name, color, false, 0, true)
	return
}

func (c *cardinal) fetchRole(roleName string, guild *discordgo.Guild, color int) (role *discordgo.Role, err error) {
	for _, r := range guild.Roles {
		if r.Name == roleName {
			if r.Permissions != 0 {
				return nil, fmt.Errorf("%s has invalid permissions", roleName)
			} else if !r.Mentionable {
				return nil, fmt.Errorf("%s is not mentionable", roleName)
			}
			role = r
			return
		}
	}

	return c.createRole(roleName, guild, color)
}

func convertColor(colorString string) (color int, err error) {
	u, err := strconv.ParseUint(colorString, 16, 64)
	if err != nil {
		return
	}
	return int(u), nil
}

func (c *cardinal) roleHasMember(guild *discordgo.Guild, roleID string) bool {
	for _, member := range guild.Members {
		for _, rid := range member.Roles {
			if rid == roleID {
				return true
			}
		}
	}
	return false
}

func (c *cardinal) handleMessage(msg *discordgo.MessageCreate) error {
	if !strings.HasPrefix(msg.Content, "!") {
		return nil
	}

	msgSplits := strings.Split(msg.Content, " ")
	if len(msgSplits) < 2 {
		return nil
	}

	command := msgSplits[0]
	if len(command) == 0 {
		return nil
	}

	command = command[1:]

	if !isValidCommand(command) {
		log.Infof("Valid command not detected in messages: %s", msg.Content)
		return nil
	}

	args := msgSplits[1:]

	guild, err := c.getGuild(msg.Message)
	if err != nil {
		return errors.New("Unable to fetch guild")
	}

	var user *discordgo.User
	var role *discordgo.Role
	color := 0

	switch command[len(command)-2:] {
	case "me":
		if len(msg.Mentions) != 0 {
			return errors.New("Found mentions")
		}

		if len(args) == 2 {
			color, err = convertColor(args[1])
		}

		user = msg.Author
		role, err = c.fetchRole(args[0], guild, color)
	case "em":
		if len(msg.Mentions) != 1 {
			return errors.New("No mentions in message")
		}

		if len(args) == 3 {
			color, err = convertColor(args[2])
		}

		user = msg.Mentions[0]
		role, err = c.fetchRole(args[1], guild, color)
	}

	if err != nil { // Role has invalid permissions
		return err
	}

	if command[0] == '!' {
		err = c.s.GuildMemberRoleRemove(guild.ID, user.ID, role.ID)

		if err == nil && !c.roleHasMember(guild, role.ID) {
			err = c.s.GuildRoleDelete(guild.ID, role.ID)
		}
	} else {
		err = c.s.GuildMemberRoleAdd(guild.ID, user.ID, role.ID)
	}

	return err
}

func (c *cardinal) messageCreate(_ *discordgo.Session, msg *discordgo.MessageCreate) {

	var status string
	err := c.handleMessage(msg)

	if err != nil {
		log.Error(err)
		status = err.Error()
	}

	c.s.UpdateStatus(0, status)
}
