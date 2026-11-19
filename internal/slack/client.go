package slack

import (
	"errors"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/slack-go/slack"
)

type SlackClient struct {
	Client *slack.Client
}

func Client(ApiKey string) (*SlackClient, error) {

	if ApiKey == "" {
		message := "Empty SLACK_BOT_TOKEN"
		logrus.WithField("service", "slack").Error(message)
		return nil, errors.New(message)
	}

	client := slack.New(ApiKey)
	return &SlackClient{
		Client: client,
	}, nil
}

func (client *SlackClient) GetUserByEmail(userEmail string) (*slack.User, error) {

	user, err := client.Client.GetUserByEmail(userEmail)
	if err != nil {
		logrus.WithField("service", "slack").Error(err)
		return nil, err
	}
	logrus.WithField("service", "slack").Info("Get user:" + user.Name + " " + user.ID)
	return user, nil
}

func (c *SlackClient) UpdateUserGroupMembers(groupId string, userIds []string) {
	if groupId == "" {
		logrus.WithField("service", "slack").Warn("SLACK_GROUP_ID_UPDATE_ONCALL vazio, grupo não atualizado")
		return
	}

	users := strings.Join(userIds, ",")
	group, err := c.Client.UpdateUserGroupMembers(groupId, users)
	if err != nil {
		logrus.WithField("service", "slack").Error(err)
		return
	}

	logrus.WithField("service", "slack").Infof("Atualizado grupo %s (%d membros)", group.Name, len(userIds))
}

func MsgOptionBlocks(blocks ...slack.Block) slack.MsgOption {
	return slack.MsgOptionBlocks(blocks...)
}

// ChannelMsgOptions posts Block Kit without Slack link/media unfurls (no preview cards).
func ChannelMsgOptions(blocks ...slack.Block) []slack.MsgOption {
	return []slack.MsgOption{
		slack.MsgOptionBlocks(blocks...),
		slack.MsgOptionDisableLinkUnfurl(),
		slack.MsgOptionDisableMediaUnfurl(),
	}
}

type MySlackBlock struct {
	BlockType   slack.MessageBlockType
	ElementType slack.MessageObjectType
	Text        string
}

func CreateSectionBlock(text string, blockType slack.MessageBlockType, elementType slack.MessageObjectType) MySlackBlock {
	return MySlackBlock{
		blockType,
		elementType,
		text,
	}
}

func ConvertToSlackBlocks(blocks []MySlackBlock) []slack.Block {
	slackBlocks := []slack.Block{}
	for _, block := range blocks {
		if block.BlockType == slack.MBTDivider {
			slackBlocks = append(slackBlocks, slack.NewDividerBlock())
			continue
		}

		textBlock := slack.NewTextBlockObject(string(block.ElementType), block.Text, false, false)
		switch block.BlockType {
		case slack.MBTSection:
			sectionBlock := slack.NewSectionBlock(textBlock, nil, nil)
			slackBlocks = append(slackBlocks, sectionBlock)
		case slack.MBTHeader:
			headerBlock := slack.NewHeaderBlock(textBlock)
			slackBlocks = append(slackBlocks, headerBlock)
		case slack.MBTContext:
			sectionBlock := slack.NewContextBlock("", textBlock)
			slackBlocks = append(slackBlocks, sectionBlock)
		default:
			// ...
		}
	}
	return slackBlocks
}
