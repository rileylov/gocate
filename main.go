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

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var baseStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("#3e6589"))

type model struct {
	table                              table.Model
	textInput                          textinput.Model
	searchQuery, statusMessage, output string
	siUnit                             bool
	width, height                      int
	itemLimit, visibleRows             int
	lastItemLimit                      int
	lastQuery                          string
}

type searchResultsMsg struct {
	query string
	limit int
	rows  []table.Row
	err   error
}

type updateDBMsg struct {
	err error
}

func main() {
	t := table.New(
		table.WithColumns([]table.Column{
			{Title: "Filename", Width: 40},
			{Title: "Path", Width: 90}, {Title: "Size", Width: 10},
			{Title: "Modified Time", Width: 20},
		}),
		table.WithFocused(true),
		table.WithHeight(30),
	)
	s := table.DefaultStyles()
	s.Header = s.Header.BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).BorderBottom(true).Bold(false)
	s.Selected = s.Selected.Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("#3e6589")).Bold(false)
	t.SetStyles(s)

	ti := textinput.New()
	ti.Placeholder = "Search for anything..."
	ti.Focus()
	ti.CharLimit = 128
	ti.Width = 30

	m := model{table: t, textInput: ti, itemLimit: 30, visibleRows: 30}
	result, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	if err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}

	if fm, ok := result.(model); ok && fm.output != "" {
		fmt.Println(fm.output) // if cannot copy to user's clipboard, exit and print the path
	}
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) View() string {
	return baseStyle.Width(m.width-2).MaxWidth(m.width).Render(
		m.textInput.View()+"\n\n"+m.table.View()+"\n\n"+m.statusMessage,
	) + "\n"
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg: // handle resizing
		m.width, m.height = msg.Width, msg.Height
		tableHeight := max(m.height-8, 1)
		m.table.SetHeight(tableHeight)
		m.visibleRows = tableHeight

		contentWidth := m.width - 2
		available := max(contentWidth-5*2-32, 20)
		nameWidth := available * 30 / 100
		m.table.SetColumns([]table.Column{
			{Title: "", Width: 2},
			{Title: "Filename", Width: nameWidth},
			{Title: "Path", Width: max(available-nameWidth, 10)},
			{Title: "Size", Width: 10},
			{Title: "Modified Time", Width: 20},
		})
		m.textInput.Width = contentWidth - 2

	case tea.KeyMsg: // handle keyboard input
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "ctrl+s":
			m.siUnit = !m.siUnit
			m.lastQuery = ""
		case "ctrl+u":
			c := exec.Command("bash", "-c", updatedbCommand)
			return m, tea.ExecProcess(c, func(err error) tea.Msg {
				return updateDBMsg{err}
			})
		case "enter":
			if row := m.table.SelectedRow(); row != nil {
				err := clipboard.WriteAll(row[2])
				if err != nil { // if user doesn't have wl-clipboard, xsel or xclip
					m.output = row[2]
				}
			}
			return m, tea.Quit
		case "alt+ctrl+h": // need to test this on more systems (seems to be ctrl+alt+backspace on mine?)
			m.textInput.SetValue("")
			m.searchQuery = ""
			m.table.SetRows([]table.Row{})
		}

	case updateDBMsg:
		if msg.err != nil {
			m.statusMessage = fmt.Sprintf("Failed to update DB: %v", msg.err)
		} else {
			m.statusMessage = "Updated DB!"
		}

	case searchResultsMsg:
		if msg.query == m.searchQuery {
			if msg.err != nil {
				m.statusMessage = msg.err.Error()
			} else {
				m.table.SetRows(msg.rows)
				m.statusMessage = fmt.Sprintf("Limit %d results", len(msg.rows))
			}
		}
	}

	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	cmds = append(cmds, cmd)

	m.searchQuery = m.textInput.Value()

	if m.table.Cursor() == m.itemLimit-1 {
		m.itemLimit = m.table.Cursor() + m.visibleRows
	}

	if m.searchQuery != m.lastQuery {
		m.itemLimit = m.visibleRows
		m.table.SetCursor(0)
	}

	if m.searchQuery != "" && (m.itemLimit != m.lastItemLimit || m.searchQuery != m.lastQuery) {
		m.lastQuery = m.searchQuery
		m.lastItemLimit = m.itemLimit
		cmds = append(cmds, runSearch(m.searchQuery, m.itemLimit, m.siUnit))
	}

	m.table, cmd = m.table.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func runSearch(query string, limit int, siUnit bool) tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("plocate", "-l", strconv.Itoa(limit), query)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			if stderr.Len() > 0 {
				return searchResultsMsg{query: query, limit: limit, err: fmt.Errorf("%s", stderr.String())}
			}
			return searchResultsMsg{query: query, limit: limit, rows: []table.Row{}}
		}

		var rows []table.Row
		for _, item := range strings.Split(stdout.String(), "\n") {
			if item == "" {
				continue
			}
			icon, size, mod := "📄", "", ""
			info, err := os.Stat(item)
			if err != nil {
				continue
			}
			if info.IsDir() {
				icon = "📂"
			} else {
				if info.Mode().Perm()&0111 != 0 {
					icon = "🔧"
				}
				size = formatSize(info.Size(), siUnit)
				mod = info.ModTime().Format("2006-01-02 15:04:05")
			}
			switch filepath.Ext(item) {
			case ".zip", ".gz", ".7z":
				icon = "📦"
			case ".png", ".jpg", ".webp", ".jpeg":
				icon = "🎨"
			case ".mp4", ".mov":
				icon = "📹"
			}
			rows = append(rows, table.Row{icon, filepath.Base(item), item, size, mod})
		}
		return searchResultsMsg{query: query, limit: limit, rows: rows}
	}
}

func formatSize(b int64, si bool) string {
	if b == 0 {
		return "0 B"
	}
	unit, suffixes := 1024.0, []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB", "EiB"}
	if si {
		unit, suffixes = 1000, []string{"B", "KB", "MB", "GB", "TB", "PB", "EB"}
	}
	i := math.Floor(math.Log(float64(b)) / math.Log(unit))
	return fmt.Sprintf("%.2f %s", float64(b)/math.Pow(unit, i), suffixes[int(i)])
}

const updatedbCommand = `sudo -v || exit 1
sudo updatedb &
PID=$!
CHARS='⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏'
while kill -0 $PID 2>/dev/null; do
  for (( i=0; i<${#CHARS}; i++ )); do
    printf "\r\033[32mUpdating DB %s\033[0m" "${CHARS:$i:1}"
    sleep 0.05
    kill -0 $PID 2>/dev/null || break
  done
done
wait $PID` // run sudo updatedb and display a loading bar
