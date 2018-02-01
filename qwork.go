package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/jagipson/refmt"
)

type UnixTime struct {
	time.Time
}

func (ut *UnixTime) UnmarshalJSON(b []byte) (err error) {
	s := strings.Trim(string(b), "\"")
	if s == "null" {
		ut.Time = time.Time{}
		return
	}
	timei, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		log.Fatal(err)
	}
	ut.Time = time.Unix(timei, 0)
	return
}

func (ut *UnixTime) MarshalJSON() ([]byte, error) {
	if ut.Time.UnixNano() == (time.Time{}).UnixNano() {
		return []byte("null"), nil
	}
	return []byte(fmt.Sprintf("\"%i\"", ut.Time.Unix())), nil
}

func (ut *UnixTime) IsSet() bool {
	return ut.UnixNano() != time.Time{}.UnixNano()
}

type Recipient struct {
	Address string `json:"address"`
	Reason  string `json:"delay_reason"`
}

type Message struct {
	Queue      string      `json:"queue_name"`
	Id         string      `json:"queue_id"`
	Time       UnixTime    `json:"arrival_time"`
	Size       int         `json:"message_size"`
	Sender     string      `json:"sender"`
	Recipients []Recipient `json:"recipients"`
}

func (m Message) QueueChar() string {
	switch m.Queue {
	case "deferred":
		return " "
	case "active":
		return "*"
	case "hold":
		return "!"
	default:
		return m.Queue
	}
}

type Messages []Message

func (m Messages) Menu() (Message, error) {
Restart:
	ui := bufio.NewReader(os.Stdin)
	if len(m) == 0 {
		return Message{}, fmt.Errorf("There are no messages to choose from.")
	}
	for k, msg := range m {
		fmt.Printf("%4d  %s%s  %s  %s\n", k+1, msg.Id, msg.QueueChar(), msg.Recipients[0].Address, msg.Time)
	}
	for {
		fmt.Printf("Select a message by entering a number from 1 to %d. Enter 0 to cancel.\n", len(m))
		answer, err := ui.ReadString('\n')
		if err != nil {
			goto Restart
		}
		ianswer, err := strconv.ParseUint(strings.TrimSpace(answer), 10, 64)
		if err != nil {
			goto Restart
		}
		if err == nil && ianswer > 0 && ianswer <= uint64(len(m)) {
			return m[ianswer-1], nil
		} else if ianswer == 0 {
			return Message{}, nil
		}
	}
}

func fetchQueue() Messages {
	postqueue, err := exec.LookPath("postqueue")
	if err != nil {
		log.Fatal(err)
	}

	cmdQueue := exec.Command(postqueue, "-j")
	var rawData bytes.Buffer
	cmdQueue.Stdout = &rawData

	err = cmdQueue.Run()
	if err != nil {
		log.Fatal(err)
	}

	bytes := rawData.Bytes()
	var messages Messages
	var message Message

	for _, v := range strings.Split(strings.TrimSpace(string(bytes)), "\n") {
		message = Message{}
		err = json.Unmarshal([]byte(v), &message)
		if err != nil {
			log.Fatal(err)
		}
		messages = append(messages, message)
	}
	return messages
}

// Returns the array index selected (which is usally one less than word)
func menu(prompt string, items []string) (index int, word string) {
	ui := bufio.NewReader(os.Stdin)
	for k, v := range items {
		fmt.Printf("%3d\t%s\n", k+1, v)
	}
	fmt.Printf("%s ", prompt)
	answer, _ := ui.ReadString('\n')
	ianswer, err := strconv.ParseUint(strings.TrimSpace(answer), 10, 64)
	if err != nil || ianswer == 0 || int(ianswer) > len(items) {
		return -1, answer
	}
	return int(ianswer - 1), answer
}

var verbs = []string{"Read", "Hold", "Unhold", "Requeue", "Delete", "Quit"}

func main() {
	postsuper, err := exec.LookPath("postsuper")
	if err != nil {
		log.Fatal(err)
	}
	postcat, err := exec.LookPath("postcat")
	if err != nil {
		log.Fatal(err)
	}

	for {
		msgs := fetchQueue()

		m, err := msgs.Menu()
		if err != nil {
			log.Fatal(err)
		}
		if m.Id == "" {
			os.Exit(0)
		}

		// Print reasons
		for _, r := range m.Recipients {
			fmt.Printf("%s:\n", r.Address)
			wrap := refmt.NewStyle()
			fmt.Printf("%s\n\n", wrap.Indent(wrap.Wrap(r.Reason)))
		}

		i, _ := menu("Select action: ", verbs)
		if i < 0 {
			fmt.Printf("\n\n")
			continue
		}
		var cmd *exec.Cmd
		switch verbs[i] {
		case "Read":
			cmd = exec.Command(postcat, "-q", m.Id)
		case "Hold":
			cmd = exec.Command(postsuper, "-h", m.Id)
		case "Unhold":
			cmd = exec.Command(postsuper, "-H", m.Id)
		case "Requeue":
			cmd = exec.Command(postsuper, "-r", m.Id)
		case "Delete":
			cmd = exec.Command(postsuper, "-d", m.Id)
		case "Quit":
			os.Exit(0)
		}
		if output, err := cmd.CombinedOutput(); err != nil {
			log.Fatal(err)
		} else {
			fmt.Println(string(output))
		}
	}
}
