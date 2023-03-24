package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// Activity of Strava
type Activity struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Distance   int    `json:"distance"`
	MovingTime int    `json:"moving_time"`
	// 他にも多数のフィールドが存在する
}

// SlackMessage represents a message to be sent to Slack
type SlackMessage struct {
	Text string `json:"text"`
}

// postToSlack sends a message to the specified Slack webhook URL
func postToSlack(webhookURL string, message *SlackMessage) error {
	payload, err := json.Marshal(message)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", webhookURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}

	client := new(http.Client)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to post to Slack: status code %d", resp.StatusCode)
	}

	return nil
}

func generateSlackMessage(activity *Activity) *SlackMessage {
	message := &SlackMessage{
		Text: fmt.Sprintf("New activity logged: %s (%.2f km, %d min)", activity.Name, float64(activity.Distance)/1000, activity.MovingTime/60),
	}
	return message
}

func getAccessToken(clientID string, clientSecret string, code string) (token string, err error) {
	tokenURL := "https://www.strava.com/oauth/token"

	values := url.Values{}
	values.Set("client_id", clientID)
	values.Add("client_secret", clientSecret)
	values.Add("code", code)
	values.Add("grant_type", "authorization_code")

	req, _ := http.NewRequest("POST", tokenURL, strings.NewReader(values.Encode()))

	// Content-Type 設定
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := new(http.Client)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}

	defer func() {
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
	}()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var accessToken struct {
		Token string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &accessToken); err != nil {
		return "", err
	}

	return accessToken.Token, nil
}

func getLatestActivity(accessToken string) (activity *Activity, err error) {
	activityURL := "https://www.strava.com/api/v3/athlete/activities?per_page=1"
	req, _ := http.NewRequest("GET", activityURL, nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := new(http.Client)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer func() {
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
	}()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var activities []*Activity
	if err := json.Unmarshal(body, &activities); err != nil {
		return nil, err
	}

	if len(activities) == 0 {
		return nil, fmt.Errorf("no activities found")
	}

	return activities[0], nil
}

func exec(clientID string, clientSecret string, code string, webhookURL string) error {
	resp, err := getAccessToken(clientID, clientSecret, code)
	if err != nil {
		return err
	}

	activity, err := getLatestActivity(resp)
	if err != nil {
		return err
	}

	message := generateSlackMessage(activity)

	err = postToSlack(webhookURL, message)
	if err != nil {
		return err
	}

	return nil
}

func main() {
	if err := exec(os.Getenv("CLIENT_ID"), os.Getenv("CLIENT_SECRET"), os.Getenv("CODE"), os.Getenv("WEBHOOK_URL")); err != nil {
		log.Println(err)
		os.Exit(1)
	}
	os.Exit(0)
}
