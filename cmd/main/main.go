package main

import (
	"errors"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"

	"github.com/bwmarrin/discordgo"
	log "github.com/sirupsen/logrus"
)

var consecutiveWhiteSpacePattern *regexp.Regexp

func init() {
	consecutiveWhiteSpacePattern = regexp.MustCompile(`[\s\p{Zs}]{2,}`)
}

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
		"!em",
		"who",
		"!who":
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

func (c *cardinal) fetchRoleMembers(roleName string, guild *discordgo.Guild) (message string, err error) {
	var role *discordgo.Role
	for _, r := range guild.Roles {
		if r.Name == roleName {
			if !r.Mentionable {
				return "", fmt.Errorf("%s is not mentionable", roleName)
			}
			role = r
		}
	}
	message = "User(s) in role " + roleName + ":\n"
	for _, m := range guild.Members {
		for _, r := range m.Roles {
			if role.ID == r {
				message += m.User.Username + "\n"
			}
		}
	}
	return message, nil
}

func (c *cardinal) roleExists(guild *discordgo.Guild, roleName string) bool {
	for _, r := range guild.Roles {
		if r.Name == roleName {
			return true
		}
	}
	return false
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
	if !strings.HasPrefix(msg.Content, "!") && !msg.Author.Bot {
		return nil
	}

	msg.Content = consecutiveWhiteSpacePattern.ReplaceAllString(msg.Content, " ")

	msgSplits := strings.Split(msg.Content, " ")
	if len(msgSplits) == 0 {
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
	var roleID string
	var colorString string
	command = strings.TrimSpace(command)

	switch command {
	case "me":
		if len(msg.Mentions) != 0 {
			return errors.New("Found mentions")
		}

		if len(args) < 1 {
			return errors.New("not enough args")
		}

		if len(args) >= 2 {
			colorString = args[1]
		}

		user = msg.Author
		roleID = args[0]
	case "em":
		if len(msg.Mentions) != 1 {
			return errors.New("Invalid number of mentions")
		}

		if len(args) < 2 {
			return errors.New("not enough args")
		}

		if len(args) >= 3 {
			colorString = args[2]
		}

		user = msg.Mentions[0]
		roleID = args[1]
	case "who":
		if len(msg.Mentions) != 0 {
			return errors.New("Invalid number of mentions")
		}

		if len(args) < 1 {
			return errors.New("Not enough args")
		}

		if len(args) > 1 {
			return errors.New("too many args")
		}

		roleID = args[0]
		if !c.roleExists(guild, roleID) {
			res := roleID + " is not an existing role. `!who` is caps sensitive."
			c.s.ChannelMessageSend(msg.Message.ChannelID, res)
			return errors.New(roleID + " is not an existing role")
		}

		res, err := c.fetchRoleMembers(roleID, guild)
		c.s.ChannelMessageSend(msg.Message.ChannelID, res)
		return err
	}

	var color int

	if colorString == "" {
		color = rand.Intn(16777215)
	} else {
		color, err = convertColor(colorString)
		if err != nil {
			return errors.New("Invalid hex color")
		}
	}

	role, err := c.fetchRole(roleID, guild, color)

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
