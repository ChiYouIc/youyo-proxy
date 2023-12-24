package youyoproxy

import (
	"testing"
)

func TestLog_InfoF(t *testing.T) {
	proxy := HttpProxy{}
	proxy.Info("hahaha %s %s", "ss", "cc")
}

func TestLog_WarnF(t *testing.T) {
	proxy := HttpProxy{}
	proxy.Warn("waring")
}

func TestLog_Info_Warn(t *testing.T) {
	proxy := HttpProxy{}
	proxy.Info("hahaha")
	proxy.Warn("waring")
}
