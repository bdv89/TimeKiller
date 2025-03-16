package main

import (
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// TimerService interface for abstraction
type TimerService interface {
	Start()
	Stop()
	GetRemainingTime() time.Duration
	GetEndTime() time.Time
}

// RealTimer implements TimerService
type RealTimer struct {
	duration time.Duration
	endTime  time.Time
	timer    *time.Timer
	stopped  bool
	done     chan struct{}
}

func NewRealTimer(duration time.Duration) *RealTimer {
	return &RealTimer{
		duration: duration,
		stopped:  true,
		done:     make(chan struct{}),
	}
}

func (rt *RealTimer) Start() {
	if rt.stopped {
		rt.endTime = time.Now().Add(rt.duration)
		rt.timer = time.NewTimer(rt.duration)
		rt.stopped = false
	}
}

func (rt *RealTimer) Stop() {
	if !rt.stopped {
		rt.timer.Stop()
		rt.stopped = true
		close(rt.done)
	}
}

func (rt *RealTimer) GetRemainingTime() time.Duration {
	if rt.stopped {
		return 0
	}
	return time.Until(rt.endTime)
}

func (rt *RealTimer) GetEndTime() time.Time {
	return rt.endTime
}

// GUI struct to manage the UI
type GUI struct {
	app            fyne.App
	window         fyne.Window
	timerService   TimerService
	nameEntry      *widget.Entry
	minutesEntry   *widget.Entry
	hourSetEntry   *widget.Entry
	countdownLabel *widget.Label
	endTimeLabel   *widget.Label
	startButton    *widget.Button
	stopButton     *widget.Button
}

func NewGUI(app fyne.App, timerService TimerService) *GUI {
	return &GUI{
		app:          app,
		timerService: timerService,
	}
}

func (g *GUI) createUI() {
	g.window = g.app.NewWindow("Timer")
	g.window.SetFixedSize(true)
	g.window.Resize(fyne.NewSize(400, 300))

	g.nameEntry = widget.NewEntry()
	g.nameEntry.SetPlaceHolder("NAME")
	g.nameEntry.OnChanged = func(text string) {
		g.nameEntry.Resize(fyne.NewSize(g.nameEntry.MinSize().Width+float32(len(text)*10), g.nameEntry.MinSize().Height))
	}

	g.minutesEntry = widget.NewEntry()
	g.minutesEntry.SetPlaceHolder("MINUTES")
	g.minutesEntry.Validator = func(text string) error {
		minutes, err := strconv.Atoi(text)
		if err != nil || minutes <= 0 {
			return errors.New("must be a positive integer")
		}
		return nil
	}
	g.hourSetEntry = widget.NewEntry()

	g.minutesEntry.OnSubmitted = func(text string) {
		g.startTimer()
	}

	g.nameEntry.OnSubmitted = func(text string) {
		g.startTimer()
	}

	g.hourSetEntry.OnSubmitted = func(text string) {
		g.startTimer()
	}
	g.hourSetEntry.SetPlaceHolder("HOUR SET (HHMM)")
	g.hourSetEntry.Validator = func(text string) error {
		if len(text) != 4 {
			return errors.New("must be 4 digits (HHMM)")
		}
		hour, err1 := strconv.Atoi(text[:2])
		minute, err2 := strconv.Atoi(text[2:])
		if err1 != nil || err2 != nil {
			return errors.New("must be numbers")
		}
		if hour < 0 || hour > 23 || minute < 0 || minute > 59 {
			return errors.New("invalid time (HH:00-23:59)")
		}
		return nil
	}

	g.countdownLabel = widget.NewLabel("00:00:00")
	g.endTimeLabel = widget.NewLabel("End Time: --:--")

	g.startButton = widget.NewButton("Start", g.startTimer)
	g.stopButton = widget.NewButton("Stop", g.stopTimer)
	g.stopButton.Disable()

	form := &widget.Form{
		Items: []*widget.FormItem{
			{Text: "Name", Widget: g.nameEntry},
			{Text: "Minutes", Widget: g.minutesEntry},
			{Text: "Hour Set", Widget: g.hourSetEntry},
		},
	}

	content := container.NewVBox(
		form,
		g.countdownLabel,
		g.endTimeLabel,
		container.NewHBox(g.startButton, g.stopButton),
	)

	g.window.SetContent(content)
	g.window.Canvas().SetOnTypedKey(func(key *fyne.KeyEvent) {
		if key.Name == fyne.KeyEscape {
			g.window.Close()
		}
		if key.Name == fyne.KeyReturn {
			g.startTimer()
		}
	})

	// Focus on any key press when window is active
	g.window.Canvas().SetOnTypedRune(func(r rune) {
		g.window.Canvas().Focus(g.minutesEntry)
	})

	// Focus MINUTE field on window creation and close
	g.window.Canvas().Focus(g.minutesEntry)
	g.window.SetOnClosed(func() {
		g.window.Canvas().Focus(g.minutesEntry)
	})

	g.window.Show()
}

func (g *GUI) startTimer() {
	minutesText := g.minutesEntry.Text
	hourSetText := g.hourSetEntry.Text

	var duration time.Duration
	if minutesText != "" {
		minutes, _ := strconv.ParseFloat(minutesText, 64)
		duration = time.Duration(minutes) * time.Minute
	} else if hourSetText != "" {
		hour, _ := strconv.Atoi(hourSetText[:2])
		minute, _ := strconv.Atoi(hourSetText[2:])
		now := time.Now()
		endTime := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
		if endTime.Before(now) {
			endTime = endTime.Add(24 * time.Hour)
		}
		duration = time.Until(endTime)
	} else {
		dialog.ShowError(errors.New("please enter minutes or hour set"), g.window)
		return
	}

	g.timerService = NewRealTimer(duration)
	g.timerService.Start()
	g.startButton.Disable()
	g.stopButton.Enable()

	go func() {
		for {
			select {
			case <-g.timerService.(*RealTimer).done:
				return
			case <-time.After(1 * time.Second):
				remaining := g.timerService.GetRemainingTime()
				if remaining <= 0 {
					g.timerService.Stop()
					g.startButton.Enable()
					g.stopButton.Disable()
					g.countdownLabel.SetText("00:00:00")
					g.endTimeLabel.SetText("End Time: --:--")
					g.minimizeAllWindows()
					return
				}
				g.countdownLabel.SetText(fmt.Sprintf("%02d:%02d:%02d", int(remaining.Hours()), int(remaining.Minutes())%60, int(remaining.Seconds())%60))
				g.endTimeLabel.SetText(fmt.Sprintf("End Time: %02d:%02d", g.timerService.GetEndTime().Hour(), g.timerService.GetEndTime().Minute()))
			}
		}
	}()
}

func (g *GUI) stopTimer() {
	g.timerService.Stop()
	g.startButton.Enable()
	g.stopButton.Disable()
	g.countdownLabel.SetText("00:00:00")
	g.endTimeLabel.SetText("End Time: --:--")
}

func (g *GUI) minimizeAllWindows() {
	cmd := exec.Command("powershell", "-Command", "(New-Object -ComObject Shell.Application).MinimizeAll()")
	err := cmd.Run()
	if err != nil {
		fmt.Println("Error minimizing windows:", err)
	}
}

func main() {
	app := app.New()
	app.Settings().SetTheme(theme.DarkTheme())

	gui := NewGUI(app, NewRealTimer(0))
	gui.createUI()

	app.Run()
}
