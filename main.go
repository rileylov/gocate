package main

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var itemLimit = 30
var visibleRows = 30

var lastItemLimit = 0
var lastQuery = ""
var index = 0

var baseStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("240"))

type dbUpdateMsg struct {
	err error
}

type updateCountMsg struct {
	err   error
	count int
}

type debouncedCountTriggerMsg struct{}

func runUpdatedb() tea.Msg {
	cmd := exec.Command("sudo", "updatedb")
	err := cmd.Run()
	return dbUpdateMsg{err}
}

func runUpdateCount(searchQuery string) tea.Msg {
	if searchQuery == "" {
		searchQuery = "." // ermmm...
	}
	cmd := exec.Command("plocate", "-c", "-0", searchQuery)
	output, err := cmd.Output()
	if err != nil {
		return updateCountMsg{err, 0}
	}
	count, err := strconv.Atoi(strings.TrimSpace(string(output)))
	if err != nil {
		return updateCountMsg{err, 0}
	}

	return updateCountMsg{nil, count}
}

type model struct {
	table           table.Model
	textInput       textinput.Model
	searchQuery     string
	statusMessage   string
	siUnit          bool
	itemCount       int
	lastTyped       time.Time
	debounceRunning bool
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
	)
}

func debounceCmd(duration time.Duration) tea.Cmd {
	return tea.Tick(duration, func(t time.Time) tea.Msg {
		return debouncedCountTriggerMsg{}
	})
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if m.table.Focused() {
				m.table.Blur()
			} else {
				m.table.Focus()
			}
		case "alt+c": // copying this to clipboard is a temp solution, ideally after exiting i would actually like to change the directory but probably need an external script for that!
			info, err := os.Stat(m.table.SelectedRow()[2])
			if err != nil {
				m.statusMessage = fmt.Sprintf("os.Stat error: %v", err)
			}
			if info != nil && info.IsDir() {

				cmd := exec.Command("ghostty", "-e", "cd", m.table.SelectedRow()[2])
				err := cmd.Run()
				if err != nil {
					fmt.Println("Error spawning terminal:", err)
				}
				return m, tea.Quit
			}
		case "ctrl+c":
			return m, tea.Quit
		case "ctrl+s":
			m.siUnit = !m.siUnit
			lastQuery = ""
		case "ctrl+u":
			m.statusMessage = "Updating DB..."
			return m, func() tea.Msg {
				return runUpdatedb()
			}
		case "enter":
			err := clipboard.WriteAll(m.table.SelectedRow()[2]) // wl-copy does not work as root (sudo)
			if err != nil {
				m.statusMessage = fmt.Sprintf("Couldn't write to clipboard: %v", err)
			}
			return m, tea.Quit
		case "alt+ctrl+h": // apparently ctrl+alt+backspace on my keyboard ???
			m.textInput.SetValue("")
			m.searchQuery = ""
		}

	case dbUpdateMsg:
		if msg.err != nil {
			m.statusMessage = fmt.Sprintf("Failed to update DB: %v", msg.err)
		} else {
			m.statusMessage = "Updated DB"
		}

	case updateCountMsg:
		if msg.err != nil {
			m.itemCount = 0
		} else {
			m.itemCount = msg.count
		}
		m.debounceRunning = false

	case debouncedCountTriggerMsg:
		return m, func() tea.Msg {
			return runUpdateCount(m.textInput.Value())
		}
	}

	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	cmds = append(cmds, cmd)

	newQuery := m.textInput.Value()
	if newQuery != m.searchQuery {
		m.lastTyped = time.Now()
		if !m.debounceRunning {
			m.debounceRunning = true
			cmds = append(cmds, debounceCmd(200*time.Millisecond))
		}
	}

	m.searchQuery = newQuery

	if m.table.Cursor() == (itemLimit - 1) {
		itemLimit = m.table.Cursor() + visibleRows
	}

	if m.searchQuery != lastQuery {
		itemLimit = visibleRows
		m.table.SetCursor(0)
	}

	if m.searchQuery != "" {
		if itemLimit != lastItemLimit || m.searchQuery != lastQuery {
			index += 1

			m.statusMessage = "running plocate with: " + strconv.Itoa(itemLimit) + " limit, query count:" + strconv.Itoa(index)
			osCmd := exec.Command("plocate", "-l", strconv.Itoa(itemLimit), m.searchQuery)

			var stdoutBuf, stderrBuf bytes.Buffer
			osCmd.Stdout = &stdoutBuf
			osCmd.Stderr = &stderrBuf

			err := osCmd.Run()
			stdout := stdoutBuf.String()
			stderr := stderrBuf.String()

			if err != nil {
				if stderr != "" {
					m.statusMessage = fmt.Sprintf("Error executing query: %v", stderr)
				} else if len(stdout) < 1 {
					m.statusMessage = "No items found..."
				} else {
					m.statusMessage = fmt.Sprintf("Error executing command: %v | %v", err, []byte(stdout))
				}
			}

			items := strings.Split(stdout, "\n")
			rows := []table.Row{}
			for _, item := range items {
				if item == "" { // do not add empty items to the table
					continue
				}

				var itemType = "📄"
				var itemSize, itemModTime string

				info, err := os.Stat(item)
				if err != nil {
					m.statusMessage = fmt.Sprintf("os.Stat error: %v", err)
				}

				if info != nil {
					if info.IsDir() {
						itemType = "📂"
					} else {
						mode := info.Mode().Perm()
						if mode&0111 != 0 {
							itemType = "⚙️"
						}
						itemSize = m.readableSize(info.Size())
						itemModTime = info.ModTime().Format("2006-01-02 15:04:05")
					}

					ext := filepath.Ext(filepath.Base(item))
					switch ext {
					case ".zip", ".gz", ".tar.gz", ".7z":
						itemType = "📦"
					case ".png", ".jpg", ".webp", ".jpeg":
						itemType = "🖼️"
					case ".mp4", ".mov":
						itemType = "📹"
					}
				}

				rows = append(rows, table.Row{itemType, filepath.Base(item), item, itemSize, itemModTime})
			}

			m.table.SetRows(rows)

			lastQuery = m.searchQuery
			lastItemLimit = itemLimit
		}
	}

	m.table, cmd = m.table.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	return baseStyle.Render(
		m.textInput.View()+"\n\n"+
			m.table.View()+"\n\n"+
			"Item Count: "+strconv.Itoa(m.itemCount)+
			" | Status: "+m.statusMessage,
	) + "\n"
}

func main() {
	columns := []table.Column{
		{Title: "", Width: 2},
		{Title: "Filename", Width: 40},
		{Title: "Path", Width: 90},
		{Title: "Size", Width: 10},
		{Title: "Modified Time", Width: 20},
	}
	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(visibleRows),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)

	ti := textinput.New()
	ti.Placeholder = "Search for anything..."
	ti.Focus()
	ti.CharLimit = 64
	ti.Width = 30

	m := model{
		table:     t,
		textInput: ti,
	}

	if _, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}

}

func (m model) readableSize(bytes int64) string {
	if bytes == 0 {
		return "0 B"
	}
	if m.siUnit {
		const unit = 1000
		suffixes := []string{"B", "KB", "MB", "GB", "TB", "PB", "EB"}
		i := math.Floor(math.Log(float64(bytes)) / math.Log(unit))
		val := float64(bytes) / math.Pow(unit, i)
		return fmt.Sprintf("%.2f %s", val, suffixes[int(i)])
	} else {
		const unit = 1024
		suffixes := []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB", "EiB"}
		i := math.Floor(math.Log(float64(bytes)) / math.Log(unit))
		val := float64(bytes) / math.Pow(unit, i)
		return fmt.Sprintf("%.2f %s", val, suffixes[int(i)])
	}
}
