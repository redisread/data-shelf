package main

import (
	"fmt"
	"image/png"
	"math"
	"os"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func layoutFixture(t *testing.T) *app {
	a := testApp(t)
	for _, name := range []string{"API", "支付服务"} {
		if _, e := a.store.Create(".", name, true); e != nil {
			t.Fatal(e)
		}
	}
	for _, file := range []struct{ parent, name string }{{"API", "response.json"}, {"API", "生产环境接口异常响应与请求追踪记录-2026-11-14.json"}, {"支付服务", "重试策略.md"}, {".", "排查笔记.md"}} {
		if _, e := a.store.Create(file.parent, file.name, false); e != nil {
			t.Fatal(e)
		}
	}
	a.expanded["API"] = true
	a.expanded["支付服务"] = true
	a.refresh()
	a.open("API/response.json")
	a.active.editor.Buffer.Set("{\n  \"request_id\": \"req_20261114\",\n  \"status\": 502,\n  \"message\": \"上游服务响应超时\",\n  \"elapsed_ms\": 3012\n}")
	a.capture(a.active)
	at := strings.Index(a.active.doc.Text, "502")
	a.active.editor.Select(at, at+3)
	a.addNote()
	a.active.doc.Notes[0].Comment = "检查上游服务的超时设置，\n并对照请求日志确认重试次数。"
	a.flush()
	return a
}
func requireRect(t *testing.T, tt *ui.Tester, name string) ui.Rect {
	t.Helper()
	r, ok := tt.Find(name)
	if !ok || r.W <= 0 || r.H <= 0 {
		t.Fatalf("missing visible %q: %+v", name, r)
	}
	return r
}
func TestTreeSharedLeadingEdges(t *testing.T) {
	a := layoutFixture(t)
	tt := ui.NewTester(a.view, 1280, 820)
	api := requireRect(t, tt, "文件名：API")
	pay := requireRect(t, tt, "文件名：支付服务")
	child := requireRect(t, tt, "文件名：API/response.json")
	if api.X != pay.X || child.X-api.X != 16 {
		t.Fatalf("unaligned tree: api=%+v pay=%+v child=%+v", api, pay, child)
	}
	if api.X > 90 {
		t.Fatalf("tree names unexpectedly centered: %+v", api)
	}
	tt.SetDirection(ui.RightToLeft)
	api = requireRect(t, tt, "文件名：API")
	pay = requireRect(t, tt, "文件名：支付服务")
	child = requireRect(t, tt, "文件名：API/response.json")
	if math.Abs(float64((api.X+api.W)-(pay.X+pay.W))) > 1 || math.Abs(float64((api.X+api.W)-(child.X+child.W)-16)) > 1 {
		t.Fatalf("RTL indentation failed: %+v %+v %+v", api, pay, child)
	}
}
func TestWorkspaceSupportedSizes(t *testing.T) {
	for _, width := range []float32{1040, 1280, 1600} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			a := layoutFixture(t)
			tt := ui.NewTester(a.view, int(width), 620)
			for _, dark := range []bool{false, true} {
				tt.SetDark(dark)
				for _, label := range []string{"新建文件", "新建文件夹", "导入资料", "目录设置", "文件内容", "＋ 评论", "评论 " + a.active.doc.Notes[0].ID} {
					r := requireRect(t, tt, label)
					if r.X < 0 || r.Y < 0 || r.X+r.W > width+1 || r.Y+r.H > 621 {
						t.Fatalf("clipped %s at %v: %+v", label, width, r)
					}
				}
			}
			if os.Getenv("DATA_SHELF_SCREENSHOTS") != "" {
				for _, dark := range []bool{false, true} {
					tt.SetDark(dark)
					file, e := os.Create(fmt.Sprintf("/tmp/data-shelf-layout-%d-%t.png", int(width), dark))
					if e != nil {
						t.Fatal(e)
					}
					e = png.Encode(file, tt.Image())
					file.Close()
					if e != nil {
						t.Fatal(e)
					}
				}
			}
		})
	}
}
