package gps

import (
	"math"
)

type GPSUserData struct {
	Uid       uint64
	Latitude  float64
	Longitude float64
	Radius    float64
}

const (
	EARTH_RADIUS = 6378.137
)

func GPSCoordinateAna(gpsData []GPSUserData, threshold float64) (similar []map[uint64]float64) {
	for k := 0; k < len(gpsData); k++ {
		p1 := gpsData[k]
		for j := k + 1; j < len(gpsData); j++ {
			p2 := gpsData[j]
			d := DistanceBetweenGPS(p1.Latitude, p1.Longitude, p2.Latitude, p2.Longitude)
			//			fmt.Printf("Uid1=%v,lat1=%v,lng1=%v,Uid2=%v,lat2=%v,lng2=%v,distance=%v\n",
			//				p1.Uid, p1.Latitude, p1.Longitude,
			//				p2.Uid, p2.Latitude, p2.Longitude, d)

			//返回是千米，比较之前转换为米
			if d*1000 < threshold {
				if len(similar) == 0 {
					pairMap := make(map[uint64]float64)
					pairMap[p1.Uid] = d
					pairMap[p2.Uid] = d
					similar = append(similar, pairMap)
				} else {
					for index := 0; index < len(similar); index++ {
						gpsMap := similar[index]
						_, p1Exist := gpsMap[p1.Uid]
						_, p2Exist := gpsMap[p2.Uid]
						if p1Exist || p2Exist {
							//加入原有map
							gpsMap[p1.Uid] = d
							gpsMap[p2.Uid] = d
						} else {
							//新增一个map
							pairMap := make(map[uint64]float64)
							pairMap[p1.Uid] = d
							pairMap[p2.Uid] = d
							similar = append(similar, pairMap)
						}
					}
				}
			}
		}
	}
	return
}

//四舍五入-数学函数
func round(val float64, places int) float64 {
	var t float64
	f := math.Pow10(places)
	x := val * f
	if math.IsInf(x, 0) || math.IsNaN(x) {
		return val
	}
	if x >= 0.0 {
		t = math.Ceil(x)
		if (t - x) > 0.50000000001 {
			t -= 1.0
		}
	} else {
		t = math.Ceil(-x)
		if (t + x) > 0.50000000001 {
			t -= 1.0
		}
		t = -t
	}
	x = t / f

	if !math.IsInf(x, 0) {
		return x
	}

	return t
}

func rad(d float64) float64 {
	return d * math.Pi / 180.0
}

//Google地图GPS两点距离算法
func DistanceBetweenGPSRound(lat1 float64, lng1 float64, lat2 float64, lng2 float64) float64 {
	radLat1 := rad(lat1)
	radLat2 := rad(lat2)
	a := radLat1 - radLat2
	b := rad(lng1) - rad(lng2)
	s := 2 * math.Asin(math.Sqrt(math.Pow(math.Sin(a/2), 2)+math.Cos(radLat1)*math.Cos(radLat2)*math.Pow(math.Sin(b/2), 2)))
	s = s * EARTH_RADIUS
	s = round(s, 5)
	return s
}

//Google地图GPS两点距离算法
func DistanceBetweenGPS(lat1 float64, lng1 float64, lat2 float64, lng2 float64) float64 {
	radLat1 := rad(lat1)
	radLat2 := rad(lat2)
	a := radLat1 - radLat2
	b := rad(lng1) - rad(lng2)
	s := 2 * math.Asin(math.Sqrt(math.Pow(math.Sin(a/2), 2)+math.Cos(radLat1)*math.Cos(radLat2)*math.Pow(math.Sin(b/2), 2)))
	s = s * EARTH_RADIUS
	return s
}

type IPData struct {
	Uid       uint64
	IPAddress string
}

//IP分组
func GroupIP(IpData []IPData) (IpGroup map[string][]uint64) {
	IpGroup = make(map[string][]uint64)
	for _, v := range IpData {
		IpGroup[v.IPAddress] = append(IpGroup[v.IPAddress], v.Uid)
	}
	return
}
