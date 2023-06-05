package helper

import (
	"testing"

	"alauda.io/alb2/utils/log"
	. "alauda.io/alb2/utils/test_utils"
	"github.com/stretchr/testify/assert"
	"github.com/ztrue/tracerr"
)

func TestChart(t *testing.T) {
	base := InitBase()
	l := log.L()
	{
		chart := "registry.alauda.cn:60080/acp/chart-alauda-alb2:v3.13.0-alpha.11"
		ac, err := LoadAlbChartFromUrl(base, NewHelm(base, nil, l), chart, l)
		tracerr.Print(err)
		assert.NoError(t, err)
		imgs, err := ac.ListImage()
		assert.NoError(t, err)
		l.Info("imgs", "img", imgs)
	}
}
