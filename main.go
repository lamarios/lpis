package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/alexflint/go-arg"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
)

type Flatpak struct {
	Name      string `yaml:"name"`
	Ref       string `yaml:"ref"`
	Installed bool
}

type Script struct {
	Name     string   `yaml:"name"`
	Commands []string `yaml:"commands"`
}

type Config struct {
	Flatpaks []Flatpak `yaml:"flatpaks"`
	Scripts  []Script  `yaml:"scripts"`
}

const (
	Gnome = "gnome"
	KDE   = "kde"
)

type model struct {
	choices        []Flatpak
	scripts        []Script
	cursor         int
	selected       map[int]struct{}
	configLocation string
}

var terminal = Gnome

func initialModel(config Config, location string) model {
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
		scripts: config.Scripts,

		// A map which indicates which choices are selected. We're using
		// the  map like a mathematical set. The keys refer to the indexes
		// of the `choices` slice, above.
		selected:       selected,
		configLocation: location,
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
			m.save()
			return m, tea.Quit

		// The "up" and "k" keys move the cursor up
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		// The "down" and "j" keys move the cursor down
		case "down", "j":
			if m.cursor < len(m.choices)+len(m.scripts) {
				m.cursor++
			}

		// The "enter" key and the spacebar (a literal space) toggle
		// the selected state for the item that the cursor is pointing at.
		case "enter", " ":
			if m.cursor > len(m.choices) && m.cursor < len(m.choices)+1+len(m.scripts) {
				m.runScript(m.scripts[m.cursor-len(m.choices)-1])
			} else if m.cursor == len(m.choices) {
				m.InstallFlatpaks()
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

func (m model) runScript(script Script) {

	path, err := writeTempScript(script)
	if err != nil {
		log.Fatal(err)
	}

	command := []string{"gnome-terminal", "--", path}

	if terminal == KDE {
		command = []string{"konsole", "-e", "bash -c " + path}
	}

	cmd := exec.Command("bash", "-c", strings.Join(command, " "))

	if err := cmd.Run(); err != nil {
		log.Fatal(err)
	}

}

func (m model) InstallFlatpaks() bool {
	if len(m.selected) > 0 {
		command := []string{"gnome-terminal", "--", "flatpak", "install", "--noninteractive", "-y"}
		for k := range m.selected {
			command = append(command, m.choices[k].Ref)
		}
		cmd := exec.Command("bash", "-c", strings.Join(command, " "))

		if err := cmd.Run(); err != nil {
			log.Fatal(err)
		}

		return true
	}

	return false
}

func (m model) View() string {
	// The header
	s := "Flatpaks:\n\n"
	flatpaksOffset := len(m.choices) + 1
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
	s += fmt.Sprintf("\n%s Install missing flatpaks\n\n\n", cursor)

	if len(m.scripts) > 0 {
		s += fmt.Sprintf("Scripts:\n\n")

		for i, script := range m.scripts {

			// Is the cursor pointing at this choice?
			cursor := " " // no cursor
			if m.cursor == flatpaksOffset+i {
				cursor = ">" // cursor!
			}

			// Render the row
			s += fmt.Sprintf("%s %s\n", cursor, script.Name)
		}
	}

	// The footer
	s += "\nPress q to quit.\n"

	// Send the UI for rendering
	return s
}

func (m model) save() {
	checksum := getConfigChecksum(m.configLocation)
	configDir, err := getLocalFileLocation()

	if err != nil {
		fmt.Println("can't get local file location")
		fmt.Println(err)
	}

	f, err := os.OpenFile(configDir, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0644)
	defer f.Close()
	if err != nil {
		fmt.Println("Couldn't open" + configDir)
		fmt.Println(err)
	}
	_, err = f.WriteString(checksum)

	if err != nil {
		fmt.Println("Couldn't write to" + configDir)
		fmt.Println(err)
	}
}

func writeTempScript(script Script) (string, error) {
	path := os.TempDir() + "/" + uuid.New().String()
	f, err := os.OpenFile(path, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0744)
	defer f.Close()
	if err != nil {
		fmt.Println(err)
		return "", err
	}
	_, err = f.WriteString(strings.Join(script.Commands, "\n"))
	if err != nil {
		fmt.Println(err)
		return "", err
	}
	return path, nil
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

func getConfigChecksum(configLocation string) string {
	log.Println(configLocation)
	f, err := os.Open(configLocation)
	if err != nil {
		log.Println("yo")
		log.Fatal(err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		log.Println("yo 2")
		log.Fatal(err)
	}

	return hex.EncodeToString(h.Sum(nil))
}

func main() {

	var args struct {
		Force  bool   `arg:"-f, --force" help:"Force running the script"`
		Config string `arg:"-c, --config" help:"Config file location" default:"/usr/share/lpis/lpis.yml"`
		KDE    bool   `arg:"--kde" help:"to use KDE default terminal"`
		Gnome  bool   `arg:"--gnome" help:"to use gnome-terminal to run commands"`
	}

	arg.MustParse(&args)

	if args.KDE {
		terminal = KDE
	}

	content, err := ioutil.ReadFile(args.Config)
	if err != nil {
		// we don't have a config file, we exit
		fmt.Println("no config file, exiting")
		os.Exit(0)
	}

	if !args.Force {
		checksum := getConfigChecksum(args.Config)

		if getSavedChecksum() == checksum {
			fmt.Println("we already processed this checksum, nothing to do, use -f to skip verification")
			os.Exit(0)
		}
	}

	config := Config{}
	err = yaml.Unmarshal(content, &config)

	if err != nil {
		log.Fatal("invalid yaml file")
	}

	t := tea.NewProgram(initialModel(config, args.Config))
	if _, err := t.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
