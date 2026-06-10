package dialogs

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/jr-k/d4s/internal/ui/common"
	"github.com/jr-k/d4s/internal/ui/styles"
	"github.com/rivo/tview"
)

type PortInfo struct {
	ContainerPort uint16
	HostPort      uint16
	Protocol      string
}

type PortForwardResult struct {
	ContainerPort uint16
	HostPort      uint16
	LocalPort     uint16
	Address       string
}

func ShowPortForwardDialog(app common.AppController, containerID, containerName string, ports []PortInfo, onSubmit func(result PortForwardResult)) {
	if len(ports) == 0 {
		app.AppendFlashError("no exposed ports found")
		return
	}

	pages := app.GetPages()
	tviewApp := app.GetTviewApp()

	defaultPort := ports[0]

	// Build exposed ports display
	var exposedLines []string
	for _, p := range ports {
		exposedLines = append(exposedLines, fmt.Sprintf("%d/%s", p.ContainerPort, p.Protocol))
	}

	dialogWidth := 60
	dialogHeight := 11 + len(ports)

	// Subject line
	subject := containerName
	if len(containerID) > 12 {
		subject += fmt.Sprintf(" (%s)", containerID[:12])
	}

	// Header text
	headerText := fmt.Sprintf("[%s::b]%s[-::-]\n\n[%s]Exposed Ports:[-]", styles.TagPink, subject, styles.TagDim)
	for _, line := range exposedLines {
		headerText += fmt.Sprintf("\n[%s]  %s[-]", styles.TagIdle, line)
	}

	headerView := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetText(headerText)
	headerView.SetBackgroundColor(styles.ColorBlack)

	// Container port input
	containerPortLabel := tview.NewTextView().
		SetDynamicColors(true).
		SetText(fmt.Sprintf("[%s]Container Port:[-]", styles.TagSCKey))
	containerPortLabel.SetBackgroundColor(styles.ColorBlack)

	containerPortInput := tview.NewInputField().
		SetText(fmt.Sprintf("%d", defaultPort.ContainerPort)).
		SetFieldWidth(10)
	containerPortInput.SetFieldBackgroundColor(styles.ColorBlack).
		SetFieldTextColor(styles.ColorFg).
		SetBackgroundColor(styles.ColorBlack)

	// Local port input
	localPortLabel := tview.NewTextView().
		SetDynamicColors(true).
		SetText(fmt.Sprintf("[%s]Local Port:[-]", styles.TagSCKey))
	localPortLabel.SetBackgroundColor(styles.ColorBlack)

	localPortInput := tview.NewInputField().
		SetText(fmt.Sprintf("%d", defaultPort.ContainerPort)).
		SetFieldWidth(10)
	localPortInput.SetFieldBackgroundColor(styles.ColorBlack).
		SetFieldTextColor(styles.ColorFg).
		SetBackgroundColor(styles.ColorBlack)

	// Address input
	addrLabelView := tview.NewTextView().
		SetDynamicColors(true).
		SetText(fmt.Sprintf("[%s]Address:[-]", styles.TagSCKey))
	addrLabelView.SetBackgroundColor(styles.ColorBlack)

	addrInput := tview.NewInputField().
		SetText("localhost").
		SetFieldWidth(20)
	addrInput.SetFieldBackgroundColor(styles.ColorBlack).
		SetFieldTextColor(styles.ColorFg).
		SetBackgroundColor(styles.ColorBlack)

	// Buttons
	okBtn := tview.NewButton("Confirm")
	okBtn.SetStyle(tcell.StyleDefault.Foreground(styles.ColorFg).Background(styles.ColorBlack)).
		SetActivatedStyle(tcell.StyleDefault.Foreground(styles.ColorBlack).Background(styles.ColorMenuKey))
	okBtn.SetBackgroundColor(styles.ColorBlack)

	btnRow := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(nil, 0, 1, false).
		AddItem(okBtn, 11, 0, false).
		AddItem(nil, 0, 1, false)
	btnRow.SetBackgroundColor(styles.ColorBlack)

	// Form rows
	containerPortRow := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(containerPortLabel, 18, 0, false).
		AddItem(containerPortInput, 0, 1, true)
	containerPortRow.SetBackgroundColor(styles.ColorBlack)

	localPortRow := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(localPortLabel, 18, 0, false).
		AddItem(localPortInput, 0, 1, true)
	localPortRow.SetBackgroundColor(styles.ColorBlack)

	addrRow := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(addrLabelView, 18, 0, false).
		AddItem(addrInput, 0, 1, true)
	addrRow.SetBackgroundColor(styles.ColorBlack)

	// Layout
	content := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(nil, 1, 0, false).
		AddItem(headerView, 3+len(ports), 0, false).
		AddItem(nil, 1, 0, false).
		AddItem(containerPortRow, 1, 0, true).
		AddItem(localPortRow, 1, 0, true).
		AddItem(addrRow, 1, 0, true).
		AddItem(nil, 1, 0, false).
		AddItem(btnRow, 1, 0, false).
		AddItem(nil, 1, 0, false)
	content.SetBackgroundColor(styles.ColorBlack)

	// Modal frame
	modal := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexColumn).
			AddItem(nil, 0, 1, false).
			AddItem(content, dialogWidth, 0, true).
			AddItem(nil, 0, 1, false), dialogHeight, 0, true).
		AddItem(nil, 0, 1, false)
	modal.SetBackgroundColor(styles.ColorBlack)

	frame := tview.NewFrame(modal).
		SetBorders(0, 0, 0, 0, 0, 0)
	frame.SetBorder(true).
		SetTitle(fmt.Sprintf("[%s::b]<PortForward>[-::-]", styles.TagCyan)).
		SetTitleColor(styles.ColorTitle).
		SetBorderColor(styles.ColorMenuKey).
		SetBackgroundColor(styles.ColorBlack)

	// Center modal
	flex := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(frame, dialogHeight+2, 0, true).
			AddItem(nil, 0, 1, false), dialogWidth+4, 0, true).
		AddItem(nil, 0, 1, false)
	flex.SetBackgroundColor(styles.ColorBlack)

	close := func() {
		pages.RemovePage("portforward")
		tviewApp.SetFocus(pages)
		app.UpdateShortcuts()
	}

	submit := func() {
		cpStr := strings.TrimSpace(containerPortInput.GetText())
		lpStr := strings.TrimSpace(localPortInput.GetText())
		addr := strings.TrimSpace(addrInput.GetText())
		if addr == "" {
			addr = "localhost"
		}

		cp, err := strconv.ParseUint(cpStr, 10, 16)
		if err != nil || cp == 0 {
			app.AppendFlashError("invalid container port")
			return
		}
		lp, err := strconv.ParseUint(lpStr, 10, 16)
		if err != nil || lp == 0 {
			app.AppendFlashError("invalid local port")
			return
		}

		var hostPort uint16
		for _, p := range ports {
			if p.ContainerPort == uint16(cp) {
				hostPort = p.HostPort
				break
			}
		}

		close()
		onSubmit(PortForwardResult{
			ContainerPort: uint16(cp),
			HostPort:      hostPort,
			LocalPort:     uint16(lp),
			Address:       addr,
		})
	}

	// Focus management
	focusables := []tview.Primitive{containerPortInput, localPortInput, addrInput, okBtn}
	focusIdx := 0

	setFocusIdx := func(idx int) {
		focusIdx = idx
		tviewApp.SetFocus(focusables[idx])
	}

	frame.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEsc:
			close()
			return nil
		case tcell.KeyTab, tcell.KeyDown:
			setFocusIdx((focusIdx + 1) % len(focusables))
			return nil
		case tcell.KeyBacktab, tcell.KeyUp:
			setFocusIdx((focusIdx - 1 + len(focusables)) % len(focusables))
			return nil
		case tcell.KeyEnter:
			if focusIdx < len(focusables)-1 {
				setFocusIdx(focusIdx + 1)
			} else {
				submit()
			}
			return nil
		}
		return event
	})

	pages.AddPage("portforward", flex, true, true)
	tviewApp.SetFocus(containerPortInput)
	app.UpdateShortcuts()
}
