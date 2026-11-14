package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/egoist/mygo"
	_ "github.com/egoist/mygo/locales/zh-Hans"
	"github.com/egoist/mygo/ui"
	"github.com/redisread/data-shelf/internal/shelf"
)

func settingsPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "data-shelf", "settings.json"), nil
}
func loadRoot() (string, error) {
	if path := os.Getenv("DATA_SHELF_ROOT"); path != "" {
		return path, nil
	}
	path, err := settingsPath()
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(path)
	if err == nil {
		var cfg struct {
			Root string `json:"root"`
		}
		if err = json.Unmarshal(b, &cfg); err != nil {
			return "", err
		}
		if cfg.Root != "" {
			return cfg.Root, nil
		}
	} else if !os.IsNotExist(err) {
		return "", err
	}
	home, err := os.UserHomeDir()
	return filepath.Join(home, "Documents", "data-shelf"), err
}
func saveSettings(root string) error {
	path, err := settingsPath()
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	b, err := json.Marshal(struct {
		Root string `json:"root"`
	}{root})
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "settings-")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err = tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
func main() {
	mygo.App.SetLanguages("zh-CN")
	mygo.App.WhenReady(func() {
		root, err := loadRoot()
		if err != nil {
			showStartupError(err)
			return
		}
		s, err := shelf.Open(root)
		if err != nil {
			showStartupError(err)
			return
		}
		a := newApp(s)
		a.win = mygo.NewWindow(mygo.WindowOptions{Title: "data-shelf", Width: 1280, Height: 820, MinWidth: 1040, MinHeight: 620, StateKey: "main", Content: ui.View(a.view)})
		mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{{Role: mygo.RoleAppMenu}, {Label: "文件", Submenu: []*mygo.MenuItem{
			{Label: "新建文件", Accelerator: "CmdOrCtrl+N", Click: func(*mygo.MenuItem, *mygo.Window) { a.win.Update(func() { a.startName("file") }) }},
			{Label: "新建文件夹", Accelerator: "CmdOrCtrl+Shift+N", Click: func(*mygo.MenuItem, *mygo.Window) { a.win.Update(func() { a.startName("folder") }) }},
			{Label: "保存", Accelerator: "CmdOrCtrl+S", Click: func(*mygo.MenuItem, *mygo.Window) { a.win.Update(func() { a.whenIdle(func() { a.flush() }) }) }},
			{Label: "导入文件…", Click: func(*mygo.MenuItem, *mygo.Window) { a.win.Update(func() { a.chooseImport(false) }) }},
		}}, {Role: mygo.RoleEditMenu}, {Role: mygo.RoleWindowMenu}}))
		done := make(chan struct{})
		closing := false
		mygo.App.OnBeforeQuit(func(e *mygo.QuitEvent) {
			if closing {
				return
			}
			e.PreventDefault()
			a.whenIdle(func() {
				if a.flush() {
					closing = true
					mygo.App.Quit()
				}
			})
		})
		a.win.OnClose(func(e *mygo.CloseEvent) {
			if closing {
				return
			}
			e.PreventDefault()
			a.whenIdle(func() {
				if a.flush() {
					closing = true
					a.win.Close()
				}
			})
		})
		a.win.OnClosed(func() { close(done); a.store.Close() })
		go func() {
			ticker := time.NewTicker(1200 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-done:
					return
				case <-ticker.C:
					a.win.Update(func() {
						if !closing {
							a.sync()
						}
					})
				}
			}
		}()
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
func showStartupError(err error) {
	mygo.Dialog.Message(mygo.MessageOptions{Type: mygo.MessageError, Message: "无法打开 data-shelf", Detail: err.Error(), Buttons: []string{"退出"}})
	mygo.App.Quit()
}
