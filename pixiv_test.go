package pixiv_plugin

import (
	"github.com/m1dsummer/whitedew"
	"testing"
)

func TestPlugin(t *testing.T) {
	w := whitedew.New()
	w.SetCQServer("http://localhost:60001")
	w.AddPlugin(PluginPixiv{})
	w.Run("/event", 60000)
}
