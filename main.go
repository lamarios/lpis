package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"gopkg.in/yaml.v3"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
)

const configLocation = "/usr/share/lpis/lpis.yml"

type Flatpak struct {
	Name      string `yaml:"name"`
	Ref       string `yaml:"ref"`
	Installed bool
}

type Config struct {
	Flatpaks []Flatpak `yaml:"flatpaks"`
}

type model struct {
	choices  []Flatpak        // items on the to-do list
	cursor   int              // which to-do list item our cursor is pointing at
	selected map[int]struct{} // which to-do items are selected
}

func initialModel(config Config) model {
	selected := make(map[int]struct{})

	out, err := exec.Command("flatpak", "list").Output()
	if err != nil {
		log.Fatal(err)
	}
	installed := string(out)
	for i, f := range config.Flatpaks {
		config.Flatpaks[i].Installed = strings.Contains(installed, f.Ref)
	}

	return model{
		// Our to-do list is a grocery list
		choices: config.Flatpaks,

		// A map which indicates which choices are selected. We're using
		// the  map like a mathematical set. The keys refer to the indexes
		// of the `choices` slice, above.
		selected: selected,
	}
}
func (m model) Init() tea.Cmd {
	// Just return `nil`, which means "no I/O right now, please."
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	// Is it a key press?
	case tea.KeyMsg:

		// Cool, what was the actual key pressed?
		switch msg.String() {

		// These keys should exit the program.
		case "ctrl+c", "q":
			save()
			return m, tea.Quit

		// The "up" and "k" keys move the cursor up
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		// The "down" and "j" keys move the cursor down
		case "down", "j":
			if m.cursor < len(m.choices) {
				m.cursor++
			}

		// The "enter" key and the spacebar (a literal space) toggle
		// the selected state for the item that the cursor is pointing at.
		case "enter", " ":
			if m.cursor == len(m.choices) {
				if m.InstallFlatpaks() {
					return m, tea.Quit
				}
			} else if !m.choices[m.cursor].Installed {

				_, ok := m.selected[m.cursor]

				if ok {
					delete(m.selected, m.cursor)
				} else {
					m.selected[m.cursor] = struct{}{}
				}
			}
		}
	}

	// Return the updated model to the Bubble Tea runtime for processing.
	// Note that we're not returning a command.
	return m, nil
}

func (m model) InstallFlatpaks() bool {
	if len(m.selected) > 0 {
		command := []string{"gnome-terminal", "--", "flatpak", "install", "--noninteractive", "-y"}
		for k := range m.selected {
			command = append(command, m.choices[k].Ref)
		}
		fmt.Println(command)
		cmd := exec.Command("bash", "-c", strings.Join(command, " "))

		if err := cmd.Run(); err != nil {
			log.Fatal(err)
		}

		save()
		return true
	}

	return false
}

func (m model) View() string {
	// The header
	s := "Flatpaks:\n\n"

	// Iterate over our choices
	for i, choice := range m.choices {

		// Is the cursor pointing at this choice?
		cursor := " " // no cursor
		if m.cursor == i {
			cursor = ">" // cursor!
		}

		// Is this choice selected?
		checked := " " // not selected
		if m.choices[i].Installed {
			checked = "i" // already installed, cannot remove
		} else if _, ok := m.selected[i]; ok {
			checked = "x" // selected!
		}

		// Render the row
		s += fmt.Sprintf("%s [%s] %s\n", cursor, checked, choice.Name)
	}
	cursor := " " // no cursor
	if m.cursor == len(m.choices) {
		cursor = ">" // cursor!
	}
	s += fmt.Sprintf("%s Install missing flatpaks\n", cursor)
	// The footer
	s += "\nPress q to quit.\n"

	// Send the UI for rendering
	return s
}

func save() {
	checksum := getConfigChecksum()
	configDir, err := getLocalFileLocation()
	f, err := os.OpenFile(configDir, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println(err)
	}
	_, err = f.WriteString(checksum)
	fmt.Println(configDir)
	if err != nil {
		fmt.Println(err)
	}
}

func getLocalFileLocation() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	return configDir + "/lpis", nil
}

func getSavedChecksum() string {
	configDir, err := getLocalFileLocation()
	if err != nil {
		fmt.Println(err)
	}

	savedChecksumBytes, err := ioutil.ReadFile(configDir)
	savedChecksum := ""

	if err == nil {
		savedChecksum = string(savedChecksumBytes)
	}

	return savedChecksum
}

func getConfigChecksum() string {
	f, err := os.Open(configLocation)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		log.Fatal(err)
	}

	return hex.EncodeToString(h.Sum(nil))
}

func main() {
	content, err := ioutil.ReadFile(configLocation)

	if err != nil {
		// we don't have a config file, we exit
		fmt.Println("no config file, exiting")
		os.Exit(0)
	}

	checksum := getConfigChecksum()

	if getSavedChecksum() == checksum {
		fmt.Println("we already processed this checksum, nothing to do")
		os.Exit(0)
	}

	config := Config{}
	err = yaml.Unmarshal(content, &config)

	if err != nil {
		log.Fatal("invalid yaml file")
	}

	p := tea.NewProgram(initialModel(config))
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
