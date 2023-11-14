package main

import (
	"fmt"
	"os"
	"log"
	"context"
	"errors"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/socketmode"
	"github.com/slack-go/slack/slackevents"
)

func main() {
	godotenv.Load(".env")
	
	token := os.Getenv("SLACK_AUTH_TOKEN")
	appToken := os.Getenv("SLACK_APP_TOKEN")
	//channelID := os.Getenv("SLACK_CHANNEL_ID")
	debug := os.Getenv("BOT_DEBUG_MODE") == "true"

	client := slack.New(token, slack.OptionDebug(debug), slack.OptionAppLevelToken(appToken))

	socketClient := socketmode.New(
		client,
		socketmode.OptionDebug(debug),
		socketmode.OptionLog(log.New(os.Stdout, "socketmode: ", log.Lshortfile|log.LstdFlags)),
	)

	ctx, cancel := context.WithCancel(context.Background())

	defer cancel()

	go func(ctx context.Context, client *slack.Client, socketClient *socketmode.Client) {
		for {
			select {
			case <-ctx.Done():
				log.Println("Shutting down socketmode listener")
				return
			case event := <-socketClient.Events:
				switch event.Type {
				case socketmode.EventTypeEventsAPI:
					eventsAPIEvent, ok := event.Data.(slackevents.EventsAPIEvent)
					if !ok {
						log.Printf("Could not type cast the event to the EventsAPIEvent: %v\n", event)
						continue
					}
					socketClient.Ack(*event.Request)
					
					err := handleEventMessage(eventsAPIEvent, client)

					if err != nil {
						log.Fatal(err)
					}
				case socketmode.EventTypeSlashCommand:
					command, ok := event.Data.(slack.SlashCommand)

					if !ok {
						log.Printf("Could not type cast the message to a SlackCommant: %v\n", command)
					}

					socketClient.Ack(*event.Request)

					_, err := handleSlashCommand(command, client)
					if err != nil {
						log.Fatal(err)
					}
				}

			}
		}
	}(ctx, client, socketClient)

	socketClient.Run()
}

func handleAppMentionEvent(event *slackevents.AppMentionEvent, client *slack.Client) error {

	user, err := client.GetUserInfo(event.User)
	if err != nil {
		return err
	}
	text := strings.ToLower(event.Text)

	attachment := slack.Attachment{}
	attachment.Fields = []slack.AttachmentField{
		{
			Title: "Date",
			Value: time.Now().String(),
		}, {
			Title: "Initializer",
			Value: user.Name,
		},
	}
	if strings.Contains(text, "hello") {
		attachment.Text = fmt.Sprintf("Hello %s", user.Name)
		attachment.Color = "#4af030"
	} else {
		attachment.Text = fmt.Sprintf("How can I help you @%s?", user.Name)
		attachment.Color = "#3d3d3d"
	}
	
	_, _, err = client.PostMessage(event.Channel, slack.MsgOptionAttachments(attachment))
	if err != nil {
		return fmt.Errorf("failed to post message: %w", err)
	}
	return nil
}

func handleEventMessage(event slackevents.EventsAPIEvent, client *slack.Client) error {
	switch event.Type {
	case slackevents.CallbackEvent:
		innerEvent := event.InnerEvent

		switch ev := innerEvent.Data.(type) {
		case *slackevents.AppMentionEvent:
			err := handleAppMentionEvent(ev, client)

			if err != nil {
				return err
			}
		}
	default:
		return errors.New("unsupported event type")
	}

	return nil
}

func handleSlashCommand(command slack.SlashCommand, client *slack.Client) (interface{}, error) {

	switch command.Command {
	case "/help":
		return nil, handleHelpCommand(command, client)
	default:
		return nil, handleUnexistingCommand(command, client)
	}
}

func handleHelpCommand(command slack.SlashCommand, client *slack.Client) (error) {
	attachment := slack.Attachment{}

	attachment.Fields = []slack.AttachmentField{
		{
			Title: "Date",
			Value: time.Now().String(),
		}, {
			Title: "Initiator",
			Value: command.UserName,
		},
	}

	attachment.Text = fmt.Sprintf("Hello! %s\n I can show you all your tickets with command /tickets", command.UserName)
	_, _, err := client.PostMessage(command.ChannelID, slack.MsgOptionAttachments(attachment))
	
	if err != nil {
		return fmt.Errorf("failed to post message: %w", err)
	}
	return nil
}

func handleUnexistingCommand(command slack.SlashCommand, client *slack.Client) (error) {
	attachment := slack.Attachment{}

	attachment.Fields = []slack.AttachmentField{
		{
			Title: "Date",
			Value: time.Now().String(),
		}, {
			Title: "Initiator",
			Value: command.UserName,
		},
	}

	attachment.Text = fmt.Sprintf("Hello! %s\n Sorry, but i can't do that", command.UserName)
	_, _, err := client.PostMessage(command.ChannelID, slack.MsgOptionAttachments(attachment))
	
	if err != nil {
		return fmt.Errorf("failed to post message: %w", err)
	}
	return nil
} 