package session

import (
	"bufio"
	"fmt"
	"phoenixbuilder_fyne_gui/gui/profiles/config"
	"phoenixbuilder_fyne_gui/gui/profiles/session/list_terminal"
	"phoenixbuilder_fyne_gui/gui/profiles/session/task_config"
	"phoenixbuilder_fyne_gui/gui/profiles/session/tasks"
	"phoenixbuilder_fyne_gui/gui/utils"
	"strings"
	"time"

	bot_bridge_command "phoenixbuilder/fastbuilder/commands_generator"
	bot_bridge_fmt "phoenixbuilder/bridge/bridge_fmt"
	bot_session "phoenixbuilder_fyne_gui/dedicate/fyne/session"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type LogEntry struct {
	time time.Time
	cmd string
}

type GUI struct {
	setContent   func(v fyne.CanvasObject)
	getContent   func() fyne.CanvasObject
	origContent  fyne.CanvasObject
	masterWindow fyne.Window
	app          fyne.App

	writeBackConfigFn func()
	sessionConfig     *config.SessionConfigWithName
	term              *list_terminal.Terminal
	content           fyne.CanvasObject

	loadingBar                      *widget.ProgressBarInfinite
	loadinglabel                    *widget.Label
	loadingIndicator                *fyne.Container
	cmdInputBar                     *widget.Entry
	quitButton                      *widget.Button
	createFromTemplateBtn           *widget.Button
	taskSettingsButton              *widget.Button
	handleCmdInputButton            *widget.Button
	leftKeyEntryButton              *widget.Button
	keyboardLifter                  *fyne.Container
	titleRedirectBarHiderActivated  bool
	titleRedirectBar                *widget.Entry
	titleRedirectBarLastUpdatedTime time.Time
	functionGroup                   *fyne.Container
	taskMenu                        *tasks.GUI
	taskConfigMenu                  *task_config.GUI
	alreadyClosed                   bool
	terminateChan                   chan string
	BotSession                      *bot_session.Session
	OperationLogs []*LogEntry
}

func New(config *config.SessionConfigWithName, writeBackConfigFn func()) *GUI {
	gui := &GUI{
		sessionConfig:     config,
		writeBackConfigFn: writeBackConfigFn,
		OperationLogs: make([]*LogEntry,0),
	}
	return gui
}

func (g *GUI) setLoading(hint string) {
	g.functionGroup.Hide()
	g.loadingIndicator.Show()
	g.loadingBar.Start()
	g.loadinglabel.SetText(hint)
}

func (g *GUI) doneLoading() {
	g.functionGroup.Show()
	g.loadingIndicator.Hide()
	g.loadingBar.Stop()
	//g.functionGroup.Refresh()
	//g.loadingIndicator.Refresh()
	//g.loadingBar.Refresh()
	g.content.Refresh()
}

func (g *GUI) closeGUI() {
	g.alreadyClosed = true
	g.setContent(g.origContent)
	g.BotSession.Stop()
}

func (g *GUI) sendCmd(s string) {
	s = strings.TrimSpace(s)
	fmt.Println("Cmd:", s)
	g.logOperation(s)
	g.cmdInputBar.SetText("")
	g.term.AppendNewLine(s, true)
	g.BotSession.Execute(s)
}

func (g *GUI) redirectCliOutput(s string) {
	s = strings.TrimSpace(s)
	g.term.AppendNewLine(s, false)
}

func (g *GUI) redirectTitleDisplay(s string) {
	s = strings.TrimSpace(s)
	g.titleRedirectBar.Text = s
	g.titleRedirectBarLastUpdatedTime = time.Now()
	if g.titleRedirectBar.Hidden {
		g.titleRedirectBar.Show()
		if !g.titleRedirectBarHiderActivated {
			g.titleRedirectBarHiderActivated = true
			go func() {
				for {
					time.Sleep(3 * time.Second)
					if time.Since(g.titleRedirectBarLastUpdatedTime) > time.Second*3 {
						g.titleRedirectBar.Hide()
						g.titleRedirectBarHiderActivated = false
						break
					}
				}
			}()
		}
	}
	g.titleRedirectBar.Refresh()
}

func (g *GUI) onLoginError(err error) {
	dialog.NewError(err, g.masterWindow).Show()
	g.closeGUI()
}

func (g *GUI) onRuntimeError(err error) {
	dialog.NewError(err, g.masterWindow).Show()
	g.closeGUI()
}

func (g *GUI) makeToolContent() fyne.CanvasObject {
	g.loadingBar = widget.NewProgressBarInfinite()
	g.loadinglabel = widget.NewLabel("正在加载...")
	g.loadinglabel.Alignment = fyne.TextAlignCenter
	g.loadingIndicator = container.NewVBox(
		g.loadinglabel, g.loadingBar)
	g.cmdInputBar = widget.NewEntry()
	g.cmdInputBar.PlaceHolder = "输入/黏贴命令 (中文可能有问题)"
	g.cmdInputBar.OnSubmitted = func(s string) {
		g.sendCmd(s)
	}
	g.handleCmdInputButton = &widget.Button{
		Text:          "",
		Icon:          theme.NavigateNextIcon(),
		IconPlacement: widget.ButtonIconTrailingText,
		Importance:    widget.MediumImportance,
		OnTapped: func() {
			g.sendCmd(g.cmdInputBar.Text)
		},
	}
	g.keyboardLifter = container.NewVBox()
	g.leftKeyEntryButton = &widget.Button{
		Text:       "",
		Icon:       theme.MoveUpIcon(),
		Importance: widget.MediumImportance,
		OnTapped: func() {
			if len(g.keyboardLifter.Objects) == 0 {
				g.keyboardLifter.Add(
					container.NewBorder(nil, nil, nil, nil, &widget.Button{
						Icon:       theme.MoveDownIcon(),
						Importance: widget.LowImportance,
						OnTapped: func() {
							g.keyboardLifter.Objects = make([]fyne.CanvasObject, 0)
							g.keyboardLifter.Refresh()
						},
					}),
				)
				for i := 0; i < 5; i++ {
					g.keyboardLifter.Add(widget.NewLabel(""))
				}
			} else {
				g.keyboardLifter.Add(widget.NewLabel(""))
			}
		},
	}
	var cmdInputRight *fyne.Container
	var cmdInputLeft *fyne.Container
	if fyne.CurrentDevice().IsMobile() {
		cmdInputRight = container.NewGridWithColumns(2, g.leftKeyEntryButton, g.handleCmdInputButton)
	} else {
		cmdInputRight = container.NewGridWithColumns(1, g.handleCmdInputButton)
	}
	cmdInputLeft = container.NewGridWithColumns(2, &widget.Button{
		Text:       "",
		Icon:       theme.CancelIcon(),
		Importance: widget.MediumImportance,
		OnTapped: func() {
			g.cmdInputBar.SetText("")
		},
	}, &widget.Button{
		Text:       "",
		Icon:       theme.HistoryIcon(),
		Importance: widget.MediumImportance,
		OnTapped: func() {
			g.PopupOperationLog()
		},
	})

	g.quitButton = widget.NewButton("结束会话", func() {
		g.closeGUI()
	})
	g.quitButton.Icon = theme.NavigateBackIcon()
	g.quitButton.IconPlacement = widget.ButtonIconLeadingText
	g.taskSettingsButton = widget.NewButton("任务及配置", func() {
		//g.closeGUI()
	})
	g.taskSettingsButton.Icon = theme.SettingsIcon()
	g.taskSettingsButton.IconPlacement = widget.ButtonIconLeadingText
	g.createFromTemplateBtn = widget.NewButton("可用命令", func() {})
	g.createFromTemplateBtn.Icon = theme.ContentAddIcon()
	g.createFromTemplateBtn.IconPlacement = widget.ButtonIconLeadingText
	g.createFromTemplateBtn.Importance = widget.HighImportance
	g.titleRedirectBar = widget.NewMultiLineEntry()
	g.titleRedirectBar.Disable()
	g.titleRedirectBar.Wrapping = fyne.TextWrapWord
	g.titleRedirectBar.Hide()
	g.functionGroup = container.NewVBox(
		g.titleRedirectBar,
		container.NewBorder(nil, nil,cmdInputLeft , cmdInputRight, g.cmdInputBar),
		container.NewGridWithColumns(3,
			g.quitButton, g.taskSettingsButton, g.createFromTemplateBtn,
		),
		g.keyboardLifter,
	)

	g.functionGroup.Hide()
	return container.NewVBox(g.loadingIndicator, g.functionGroup)
}

func (g *GUI) PopupOperationLog(){
	currentPage:=g.getContent()
	expandEntry:=widget.NewMultiLineEntry()
	timeLable:=widget.NewLabel("time")
	DetailHeader:=container.NewVBox(timeLable,expandEntry)
	expandEntry.Disable()
	DetailHeader.Hide()
	LogList:=widget.NewList(func() int {
		return len(g.OperationLogs)
	}, func() fyne.CanvasObject {
		dataLabel:=widget.NewLabel("cmd")
		dataLabel.Wrapping=fyne.TextTruncate
		hiddenTimeLable:=widget.NewLabel("time")
		hiddenTimeLable.Hide()
		return container.NewBorder(nil,nil,hiddenTimeLable,
			container.NewGridWithColumns(2,
				&widget.Button{
					DisableableWidget: widget.DisableableWidget{},
					Icon:              theme.ContentCopyIcon(),
					Importance:        widget.LowImportance,
					OnTapped: func() {g.masterWindow.Clipboard().SetContent(dataLabel.Text)},
				},&widget.Button{
					DisableableWidget: widget.DisableableWidget{},
					Icon:              theme.VisibilityIcon(),
					Importance:        widget.LowImportance,
					OnTapped: func() {
						if DetailHeader.Hidden {
							DetailHeader.Show()
						}
						expandEntry.SetText(dataLabel.Text)
						timeLable.SetText(hiddenTimeLable.Text)
					},
				},
			),dataLabel,
		)
	}, func(id widget.ListItemID, object fyne.CanvasObject) {
		object.(*fyne.Container).Objects[0].(*widget.Label).SetText(g.OperationLogs[id].cmd)
		////(*widget.Label).SetText(g.OperationLogs[id].time.Format("2006-01-02 15:04:05"))
		object.(*fyne.Container).Objects[1].(*widget.Label).
			SetText(g.OperationLogs[id].time.Format("2006-01-02 15:04:05"))
	})
	LogPage:=container.NewBorder(
	//&widget.Button{
	//	Text:              "insert",
	//	Icon:              theme.CancelIcon(),
	//	Importance:        widget.MediumImportance,
	//	OnTapped: func() {
	//		g.OperationLogs=append(g.OperationLogs,&LogEntry{time.Now(),"insert"})
	//		LogList.Refresh()
	//	},},
	DetailHeader,
	&widget.Button{
		Text:              "关闭",
		Icon:              theme.CancelIcon(),
		Importance:        widget.MediumImportance,
		OnTapped: func() {
			g.setContent(currentPage)
		},
	},nil,nil, LogList)
	g.setContent(LogPage)
}

func (g *GUI) GetContent(setContent func(v fyne.CanvasObject), getContent func() fyne.CanvasObject, masterWindow fyne.Window, app fyne.App) fyne.CanvasObject {
	g.origContent = getContent()
	g.setContent = setContent
	g.getContent = getContent
	g.masterWindow = masterWindow
	g.app = app
	g.term = list_terminal.New()
	g.term.OnPasteFn = func(s string) {
		g.cmdInputBar.SetText(s)
	}
	toolbar := g.makeToolContent()
	g.content = container.NewBorder(
		nil, toolbar, nil, nil,
		g.term.GetContent(g.masterWindow),
	)

	return g.content
}

func (g *GUI) AfterMount() {
	bot_bridge_fmt.HookFunc = g.redirectCliOutput
	bot_bridge_command.AdditionalChatCb = g.redirectCliOutput
	bot_bridge_command.AdditionalTitleCb = g.redirectTitleDisplay

	g.setLoading("正在登录，最长可能需要30s...")
	go func() {
		g.BotSession = bot_session.NewSession(g.sessionConfig.Config)
		if g.BotSession == nil {
			g.onLoginError(fmt.Errorf("一个现有会话未正常退出，或许你需要重启程序"))
			return
		}
		terminateChan, err := g.BotSession.Start()
		if err != nil {
			g.onLoginError(fmt.Errorf("无法顺利登陆到租赁服中\n%v", err))
			return
		}
		g.writeBackConfigFn()
		g.loadOperationLogFile()
		defer g.writeBackOperationLog()
		g.taskMenu = tasks.New(g.BotSession, g.sendCmd, g.app)
		g.createFromTemplateBtn.OnTapped = func() {
			g.setContent(g.taskMenu.GetContent(g.setContent, g.getContent, g.masterWindow))
		}
		g.taskConfigMenu = task_config.New(g.BotSession.GetEnvironment())
		g.taskSettingsButton.OnTapped = func() {
			g.setContent(g.taskConfigMenu.GetContent(g.setContent, g.getContent, g.masterWindow))
		}
		g.terminateChan = terminateChan
		g.doneLoading()
		closeReason := <-g.terminateChan
		if !g.alreadyClosed {
			g.onRuntimeError(fmt.Errorf("和租赁服的连接被迫断开了\n%v", closeReason))
			return
		}
	}()
}

func (g *GUI) loadOperationLogFile() {
	fp, err := g.app.Storage().Open("history.log")
	if err == nil {
		defer fp.Close()
		reader:=bufio.NewReader(fp)
		for {
			l,_,err:=reader.ReadLine()
			if err!=nil{
				break
			}
			ls:=strings.Split(string(l)," ")
			if len(ls)<2{
				break
			}
			t, err := time.Parse("2006-01-02/15:04:05",ls[0])
			if err != nil {
				break 
			}
			g.OperationLogs=append(g.OperationLogs,&LogEntry{
				time: t,
				cmd:  strings.Join(ls[1:]," "),
			})
		}
	}
}

func (g *GUI) logOperation(cmd string) {
	g.OperationLogs=append(g.OperationLogs,&LogEntry{time.Now(),cmd})
}

func (g *GUI) writeBackOperationLog (){
	data:=make([]string,0)
	for _,l:=range g.OperationLogs{
		data= append(data, fmt.Sprintf("%v %v",l.time.Format("2006-01-02/15:04:05"),l.cmd))
	}
	utils.WriteOrCreateFile(g.app.Storage(),"history.log",[]byte(strings.Join(data,"\n")))
}