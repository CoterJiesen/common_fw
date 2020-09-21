package gps

import (
	"testing"
)

//测试入口
func TestEntry(t *testing.T) {
	//输入十组GPS坐标和预期值，使用算法计算
	lat1 := 30.6152183040
	lng1 := 104.0669971638
	lat2 := 30.5941054726
	lng2 := 104.0544115012
	d := DistanceBetweenGPSRound(lat1, lng1, lat2, lng2)
	s := DistanceBetweenGPS(lat1, lng1, lat2, lng2)

	t.Logf("Distance=%v,s=%v", d, s)
	if d < 2.640 || d > 2.650 {
		t.FailNow()
	}
	if s < 2.640 || s > 2.650 {
		t.FailNow()
	}

	lat3 := 30.6584821400
	lng3 := 104.0659261337
	lat4 := 30.5720084690
	lng4 := 104.0376284011
	d = DistanceBetweenGPSRound(lat3, lng3, lat4, lng4)
	s = DistanceBetweenGPS(lat3, lng3, lat4, lng4)
	t.Logf("Distance=%v,s=%v", d, s)
	if d < 9.99 || d > 10.01 {
		t.FailNow()
	}
	if s < 9.99 || s > 10.01 {
		t.FailNow()
	}

	gpsData := []GPSUserData{{1, 30.6152183040, 104.0669971638, 0},
		{2, 30.5941054726, 104.0544115012, 0},
		{3, 30.6584821400, 104.0659261337, 0},
		{4, 30.5720084690, 104.0376284011, 0}}
	similar := GPSCoordinateAna(gpsData, 3000)

	t.Logf("similar=%v", similar)

	if len(similar) != 1 {
		t.FailNow()
	}

	//IP分组测试
	IpData1 := []IPData{
		{1, "192.168.7.205"},
		{2, "192.168.7.201"},
		{3, "192.168.7.202"},
		{4, "192.168.7.203"},
	}
	IPGroup1 := GroupIP(IpData1)
	t.Logf("IPGroup1=%v", IPGroup1)
	SameIPCount := 0
	for _, v := range IPGroup1 {
		if len(v) > 1 {
			SameIPCount++
		}
	}

	if SameIPCount != 0 {
		t.FailNow()
	}

	//IP分组测试
	IpData2 := []IPData{
		{1, "192.168.7.205"},
		{2, "192.168.7.204"},
		{3, "192.168.7.205"},
		{4, "192.168.7.203"},
	}
	IPGroup2 := GroupIP(IpData2)
	t.Logf("IPGroup2=%v", IPGroup2)
	SameIPCount = 0
	for _, v := range IPGroup2 {
		if len(v) > 1 {
			SameIPCount++
		}
	}

	if SameIPCount != 1 {
		t.FailNow()
	}

	//IP分组测试
	IpData3 := []IPData{
		{1, "192.168.7.205"},
		{2, "192.168.7.204"},
		{3, "192.168.7.205"},
		{4, "192.168.7.205"},
	}
	IPGroup3 := GroupIP(IpData3)
	t.Logf("IPGroup3=%v", IPGroup3)
	SameIPCount = 0
	for _, v := range IPGroup3 {
		if len(v) > 1 {
			SameIPCount++
		}
	}

	if SameIPCount != 1 {
		t.FailNow()
	}

	//IP分组测试
	IpData4 := []IPData{
		{1, "192.168.7.205"},
		{2, "192.168.7.204"},
		{3, "192.168.7.205"},
		{4, "192.168.7.204"},
	}
	IPGroup4 := GroupIP(IpData4)
	t.Logf("IPGroup4=%v", IPGroup4)
	SameIPCount = 0
	for _, v := range IPGroup4 {
		if len(v) > 1 {
			SameIPCount++
		}
	}

	if SameIPCount != 2 {
		t.FailNow()
	}
}
