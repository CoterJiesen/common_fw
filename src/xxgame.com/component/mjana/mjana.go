package mjana

import (
	"fmt"
	"github.com/golang/protobuf/proto"
	"sort"
	"time"
	"xxgame.com/types/proto/games/common"
)

//递归拆牌（ts必须提前升序排列）
func SplitOpt(ts []*common.CardInformation) (isHu bool) {
	resChan := make(chan []*common.AnalyzeResult, 10) //拆牌结果返回管道
	branches := 0                                     //拆牌分支数目

	idx := findSeq(ts)
	if idx != nil {
		//顺子
		branches++
		var left []*common.CardInformation
		var seq []*common.CardInformation
		if idx[2] == 2 {
			left = ts[3:]
			seq = ts[:1]
		} else {
			left = append(left, ts[idx[0]+1:idx[1]]...)
			left = append(left, ts[idx[1]+1:idx[2]]...)
			left = append(left, ts[idx[2]+1:]...)
			seq = append(seq, ts[idx[0]])
		}
		go subSplit(resChan, seq, nil, nil, nil, left)
	}

	hit := false

	if len(ts) >= 3 && CardEq(ts[0], ts[1]) && CardEq(ts[1], ts[2]) {
		hit = true

		//刻子
		branches++
		go subSplit(resChan, nil, nil, []*common.CardInformation{ts[0]}, nil, ts[3:])
	}

	if hit || (len(ts) >= 2 && CardEq(ts[0], ts[1])) {
		//对子
		branches++
		go subSplit(resChan, nil, []*common.CardInformation{ts[0]}, nil, nil, ts[2:])
	}

	if branches > 0 {
		//等待拆牌结果返回
		handLen := len(ts)
		factorN := ((handLen + 1) / 3) - 1
		for {
			rs := <-resChan
			for _, v := range rs {
				if len(v.GetLeft()) == 0 {
					//3n+2
					if len(v.GetSeqs())+len(v.GetThrees()) == factorN && len(v.GetTwos()) == 1 {
						isHu = true
						return
					}
				}
			}

			branches--
			if branches <= 0 {
				break
			}
		}
	}
	return
}

//递归拆牌（ts必须提前升序排列）
func Split(ts []*common.CardInformation) []*common.AnalyzeResult {
	resChan := make(chan []*common.AnalyzeResult, 10) //拆牌结果返回管道
	branches := 0                                     //拆牌分支数目

	idx := findSeq(ts)
	if idx != nil {
		//顺子
		branches++
		var left []*common.CardInformation
		var seq []*common.CardInformation
		if idx[2] == 2 {
			left = ts[3:]
			//seq = ts[:3]
			seq = ts[:1]
		} else {
			left = append(left, ts[idx[0]+1:idx[1]]...)
			left = append(left, ts[idx[1]+1:idx[2]]...)
			left = append(left, ts[idx[2]+1:]...)
			seq = append(seq, ts[idx[0]])
			//seq = append(seq, ts[idx[1]])
			//seq = append(seq, ts[idx[2]])
		}
		//go subSplit(resChan, [][]*common.CardInformation{seq}, nil, nil, nil, left)
		go subSplit(resChan, seq, nil, nil, nil, left)
	}

	hit := false

	if len(ts) >= 3 && CardEq(ts[0], ts[1]) && CardEq(ts[1], ts[2]) {
		hit = true

		//刻子
		branches++
		go subSplit(resChan, nil, nil, []*common.CardInformation{ts[0]}, nil, ts[3:])
	}

	if hit || (len(ts) >= 2 && CardEq(ts[0], ts[1])) {
		//对子
		branches++
		go subSplit(resChan, nil, []*common.CardInformation{ts[0]}, nil, nil, ts[2:])
	}

	if branches > 0 {
		//等待拆牌结果返回
		results := make([]*common.AnalyzeResult, 0, 5)
		for {
			rs := <-resChan
			results = append(results, rs...)
			branches--
			if branches <= 0 {
				break
			}
		}
		return results
	} else {
		return []*common.AnalyzeResult{&common.AnalyzeResult{Left: ts}}
	}

	panic("unreachable!")
}

//查找顺子
func findSeq(ts []*common.CardInformation) []int {
	if len(ts) < 3 {
		return nil
	}

	var diff uint32
	idx := []int{0}
	if ts[0].GetType() == common.CardType_Z {
		return nil
	}
	for i, v := range ts[1:] {
		diff = v.GetValue() - ts[idx[len(idx)-1]].GetValue()

		if v.GetType() != ts[0].GetType() || diff > 1 {
			break
		}

		if diff == 1 {
			idx = append(idx, i+1)
			if len(idx) == 3 {
				return idx
			}
		}
	}

	return nil
}

//合并拆牌结果
func merge(x *common.AnalyzeResult, xs []*common.AnalyzeResult) []*common.AnalyzeResult {
	for _, v := range xs {
		v.Seqs = append(v.Seqs, x.Seqs...)
		v.Twos = append(v.Twos, x.Twos...)
		v.Threes = append(v.Threes, x.Threes...)
	}
	return xs
}

//生成拆牌结果
//func subSplit(resChan chan []*common.AnalyzeResult, seqs [][]*common.CardInformation, twos, threes, fours, left []*common.CardInformation) {
func subSplit(resChan chan []*common.AnalyzeResult, seqs, twos, threes, fours, left []*common.CardInformation) {
	r := new(common.AnalyzeResult)
	r.Left = left
	r.Seqs = append(r.Seqs, seqs...)
	r.Twos = append(r.Twos, twos...)
	r.Threes = append(r.Threes, threes...)
	resChan <- merge(r, Split(r.Left))
}

//比较牌
func CardEq(a *common.CardInformation, b *common.CardInformation) bool {
	if a == nil || b == nil {
		return false
	}
	return a.GetType() == b.GetType() && a.GetValue() == b.GetValue()
}

//排序结构
type Cards4Sort []*common.CardInformation //玩家手牌

func (hand Cards4Sort) Len() int { return len(hand) }
func (hand Cards4Sort) Less(i, j int) bool {
	if hand[i].GetType() != hand[j].GetType() {
		return hand[i].GetType() < hand[j].GetType()
	} else {
		return hand[i].GetValue() < hand[j].GetValue()
	}
	panic("unreachable!")
}
func (hand Cards4Sort) Swap(i, j int) { hand[i], hand[j] = hand[j], hand[i] }

//排除结构
type TestCards4Sort []int32 //测试手牌

func (hand TestCards4Sort) Len() int { return len(hand) }
func (hand TestCards4Sort) Less(i, j int) bool {
	return hand[i] < hand[j]
	panic("unreachable!")
}
func (hand TestCards4Sort) Swap(i, j int) { hand[i], hand[j] = hand[j], hand[i] }

/*癞子排除算法：
对输入手牌（包含癞子）进行分析，判断癞子牌+非癞子牌，是否可能成胡（但是该函数不做成胡判定），即：
手牌万、筒、条、字4门中，任意3门满足3n（n>=0），剩余1门满足3n+2（n>=0）所需要的癞子总数小于或等于癞子牌数。
比如，手上有2个癞子，如果剩余牌中有3个万子（不需要癞子），4个条子（需要2个癞子），5个筒子（包含将，不需要癞子），这样即可能成胡。

参数：
handCards 手牌，可能包含0-4张癞子，默认已经升序排列
wildCard 癞子牌

返回值：
[4][]int32，一级索引为3n+2的门（将门），二级索引为万筒条字各自需要癞子的张数；
特别地，如果[]int32值为0，0，0，0表示每门都可以胡；
*/
func PreAna3NP2WildCards(noWildCards []*common.CardInformation,
	wildCard *common.CardInformation, wildNum int32) (menAna [4][]int32) {
	//检查癞子个数，并生成不包含癞子的手牌
	if wildNum == 0 {
		return
	}
	//万筒条字分组
	menGroup := [4][]*common.CardInformation{}
	for _, v := range noWildCards {
		if v != nil {
			menGroup[v.GetType()-1] = append(menGroup[v.GetType()-1], v)
		}
	}

	//遍历万筒条字
	for i := 0; i < len(menGroup); i++ {
		tempAna := [4]int32{}
		var needAllNum int32 = 0 //万筒条字成为3n+2所需要牌总数
		//检查其他3门是否成3n
		for j := 0; j < len(menGroup); j++ {
			if i != j {
				tempAna[j] = numToBe3N(int32(len(menGroup[j])))
				needAllNum += tempAna[j]
			}
		}
		//检查当前k门是否成3n+2
		tempAna[i] = numToBe3NPlus2(int32(len(menGroup[i])))
		needAllNum += tempAna[i]
		if needAllNum <= wildNum {
			menAna[i] = append(menAna[i], tempAna[:]...)
		} else {
			menAna[i] = []int32{}
		}
	}
	return
}

//差几张牌成为3N
func numToBe3N(length int32) (num int32) {
	num = (3 - length%3) % 3
	return
}

//差几张牌成为3N+2
func numToBe3NPlus2(length int32) (num int32) {
	num = 2 - length%3
	return
}

//从手牌中排除一个某个牌
func excludCard(cards []*common.CardInformation, symbol *common.CardInformation) (outCards []*common.CardInformation,
	excludeNum int32) {
	for _, v := range cards {
		if !CardEq(v, symbol) {
			outCards = append(outCards, v)
		} else {
			excludeNum++
		}
	}
	return
}

/*普通判胡算法：
输入：玩家手牌
输出：是否成胡*/
func AnalyzeNormalCardsOpt(handCards []*common.CardInformation) (isHu bool) {
	ha := &HuAnalyser001{AllPai: [4][10]int32{}}
	isHu = ha.Split001(handCards)
	return
}

/*普通判胡算法：
输入：玩家手牌
输出：是否成胡*/
func AnalyzeNormalCards(handCards []*common.CardInformation) (isHu bool) {
	handLen := len(handCards)
	factorN := ((handLen + 1) / 3) - 1
	anaRes := Split(handCards)
	for _, v := range anaRes {
		if len(v.GetLeft()) == 0 {
			//3n+2
			if len(v.GetSeqs())+len(v.GetThrees()) == factorN && len(v.GetTwos()) == 1 {
				isHu = true
				return
			}
			//7对
			if handLen == 14 && len(v.GetSeqs()) == 0 && len(v.GetTwos()) == 7 {
				//七对子胡牌
				isHu = true
				return
			}
		}
	}
	return
}

/*癞子判胡入口：
输入：玩家手牌、癞子牌
输出：是否成胡*/
func AnalyzeWildCards(handCards []*common.CardInformation, wildCard *common.CardInformation) (isHu bool) {
	//检查癞子个数
	noWildCards, WildNum := excludCard(handCards, wildCard)

	if WildNum == 4 {
		//四个癞子直接成胡
		isHu = true
		return
	} else if WildNum == 0 {
		//没有癞子，直接用普通折牌算法
		isHu = AnalyzeNormalCardsOpt(handCards)
		return
	} else {
		//3n+2癞子折牌算法
		isHu3nP2 := Analyze3NP2WildCardsOpt(noWildCards, wildCard, WildNum)
		//有癞子牌的情况下检查是否7对成胡
		isHu7d := is7Dui(handCards)
		if isHu3nP2 || isHu7d {
			isHu = true
		}
	}
	return
}

/*3n+2癞子胡牌算法入口：
输入：不包含癞子的手牌、癞子牌、癞子个数
输出：是否成胡
*/
func Analyze3NP2WildCards(noWildCards []*common.CardInformation,
	wildCard *common.CardInformation, wildNum int32) (isHu bool) {
	//	tThen := time.Now().Nanosecond()

	menAnaRes := PreAna3NP2WildCards(noWildCards, wildCard, wildNum)
	//	fmt.Printf("menAnaRes=%v\n", menAnaRes)
	//	tThen1 := time.Now().Nanosecond()
	//	fmt.Printf("01 Spent Time=%v\n",(tThen1-tThen))

	isHu = Ana3NP2WildMenCardsOpt(noWildCards, wildCard, wildNum, menAnaRes)
	//	tNow := time.Now().Nanosecond()
	//	fmt.Printf("02 Spent Time=%v\n",(tNow-tThen1))
	//	fmt.Printf("AnaOut=%v\n", AnaOut)

	return
}
func Analyze3NP2WildCardsOpt(noWildCards []*common.CardInformation,
	wildCard *common.CardInformation, wildNum int32) (isHu bool) {

	menAnaRes := PreAna3NP2WildCards(noWildCards, wildCard, wildNum)

	isHu = Ana3NP2WildMenCardsOptParallel(noWildCards, wildCard, wildNum, menAnaRes)

	return
}

/*
癞子胡牌算法2：根据癞子排除算法的结果，对手牌进行穷举匹配，返回可以成胡的牌型组合
*/
func Ana3NP2WildMenCardsOptParallel(noWildCards []*common.CardInformation, wildCard *common.CardInformation, wildNum int32,
	menAna [4][]int32) (isHu bool) {
	if wildNum == 0 {
		return
	}
	//区分是否有全0的结果
	allZeroIndex := []int{}
	normalIndex := []int{}
	for k := 0; k < len(menAna); k++ {
		if len(menAna[k]) != 4 {
			continue
		}
		//如果menAna是{0,0,0,0}需要通配所有门
		if menAna[k][0] == 0 && menAna[k][1] == 0 && menAna[k][2] == 0 && menAna[k][3] == 0 {
			allZeroIndex = append(allZeroIndex, k)
		} else {
			normalIndex = append(normalIndex, k)
		}
	}

	resChan := make(chan bool, 100) //等待癞子分析结果管道
	branches := 0                   //癞子分支数目

	//先分析正常的结果
	for _, v := range normalIndex {
		branches++
		idx := v
		go func() {
			isHu = Exhaustion4Wild3NP2CardsOpt(noWildCards, menAna[idx])
			resChan <- isHu
		}()
	}

	//计算已经全部分析出来的门

	//再分析全0的结果，排除上面已经分析出来的，每一门直接给3个进行匹配
	for _, v := range allZeroIndex {
		branches++
		go func() {
			isHu = ExhaustionAllZeroWild3NP2CardsOptParallel(noWildCards, menAna[v])
			resChan <- isHu
		}()
	}

	//等待结果
	if branches > 0 {
		for {
			isHu = <-resChan
			if isHu {
				return
			}
			branches--
			if branches <= 0 {
				break
			}
		}
	}

	return
}

/*
癞子胡牌算法2：根据癞子排除算法的结果，对手牌进行穷举匹配，返回可以成胡的牌型组合
*/
func Ana3NP2WildMenCardsOpt(noWildCards []*common.CardInformation, wildCard *common.CardInformation, wildNum int32,
	menAna [4][]int32) (isHu bool) {
	if wildNum == 0 {
		return
	}
	//区分是否有全0的结果
	allZeroIndex := []int{}
	normalIndex := []int{}
	for k := 0; k < len(menAna); k++ {
		if len(menAna[k]) != 4 {
			continue
		}
		//如果menAna是{0,0,0,0}需要通配所有门
		if menAna[k][0] == 0 && menAna[k][1] == 0 && menAna[k][2] == 0 && menAna[k][3] == 0 {
			allZeroIndex = append(allZeroIndex, k)
		} else {
			normalIndex = append(normalIndex, k)
		}
	}
	//	fmt.Printf("\nallZeroIndex=%v\n", allZeroIndex)
	//	fmt.Printf("normalIndex=%v\n", normalIndex)

	//	tThen := time.Now().Nanosecond()
	//先分析正常的结果
	for _, v := range normalIndex {
		isHu = Exhaustion4Wild3NP2CardsOpt(noWildCards, menAna[v])
		if isHu {
			return
		}
	}
	//	tThen1 := time.Now().Nanosecond()
	//	fmt.Printf("03 Spent Time=%v\n",(tThen1-tThen))
	//计算已经全部分析出来的门

	//再分析全0的结果，排除上面已经分析出来的，每一门直接给3个进行匹配
	for _, v := range allZeroIndex {
		isHu = ExhaustionAllZeroWild3NP2CardsOpt(noWildCards, menAna[v])
		if isHu {
			return
		}
	}
	//	tThen2 := time.Now().Nanosecond()
	//	fmt.Printf("04 Spent Time=%v\n",(tThen2-tThen1))

	//	tThen3 := time.Now().Nanosecond()
	//	fmt.Printf("05 Spent Time=%v\n",(tThen3-tThen2))
	return
}

/*
癞子胡牌算法2：根据癞子排除算法的结果，对手牌进行穷举匹配，返回可以成胡的牌型组合
*/
func Ana3NP2WildMenCards(noWildCards []*common.CardInformation, wildCard *common.CardInformation, wildNum int32,
	menAna [4][]int32) (allWildOut []*common.CardInformation) {
	if wildNum == 0 {
		return
	}
	//区分是否有全0的结果
	allZeroIndex := []int{}
	normalIndex := []int{}
	for k := 0; k < len(menAna); k++ {
		if len(menAna[k]) != 4 {
			continue
		}
		//如果menAna是{0,0,0,0}需要通配所有门
		if menAna[k][0] == 0 && menAna[k][1] == 0 && menAna[k][2] == 0 && menAna[k][3] == 0 {
			allZeroIndex = append(allZeroIndex, k)
		} else {
			normalIndex = append(normalIndex, k)
		}
	}
	//	fmt.Printf("\nallZeroIndex=%v\n", allZeroIndex)
	//	fmt.Printf("normalIndex=%v\n", normalIndex)

	tThen := time.Now().Nanosecond()
	//先分析正常的结果
	allWildCardsMap := make(map[int32]int32)
	for _, v := range normalIndex {
		wildOut := Exhaustion4Wild3NP2Cards(noWildCards, menAna[v])
		for _, key := range wildOut {
			allWildCardsMap[key] = 1
		}
	}
	tThen1 := time.Now().Nanosecond()
	fmt.Printf("03 Spent Time=%v\n", (tThen1 - tThen))
	//计算已经全部分析出来的门
	alreadyMatch := [4]int32{0, 0, 0, 0}
	allTmpWildOut := []*common.CardInformation{}
	for key, _ := range allWildCardsMap {
		allTmpWildOut = append(allTmpWildOut, &common.CardInformation{
			Type:  common.CardType(key / 10).Enum(),
			Value: proto.Uint32(uint32(key % 10)),
		})
		alreadyMatch[key/10-1]++
	}
	//	fmt.Printf("alreadyMatch=%v\n", alreadyMatch)

	//再分析全0的结果，排除上面已经分析出来的，每一门直接给3个进行匹配
	for _, v := range allZeroIndex {
		wildOut := ExhaustionAllZeroWild3NP2Cards(noWildCards, menAna[v], alreadyMatch)
		for _, key := range wildOut {
			allWildCardsMap[key] = 1
		}
	}
	tThen2 := time.Now().Nanosecond()
	fmt.Printf("04 Spent Time=%v\n", (tThen2 - tThen1))
	//整合所有结果
	for key, _ := range allWildCardsMap {
		allWildOut = append(allWildOut, &common.CardInformation{
			Type:  common.CardType(key / 10).Enum(),
			Value: proto.Uint32(uint32(key % 10)),
		})
	}
	tThen3 := time.Now().Nanosecond()
	fmt.Printf("05 Spent Time=%v\n", (tThen3 - tThen2))
	return
}

/*
癞子胡牌算法1：根据癞子排除算法的结果，对手牌进行穷举匹配，返回可以成胡的牌型组合
*/
func AnaWildMenCards(handCards []*common.CardInformation, wildCard *common.CardInformation,
	menAna [4][]int32) (allWildOut []*common.CardInformation) {
	noWildCards, WildNum := excludCard(handCards, wildCard)
	if WildNum == 0 {
		return
	}

	allZero := false
	for k := 0; k < len(menAna); k++ {
		if len(menAna[k]) != 4 {
			continue
		}
		//如果menAna是{0,0,0,0}需要通配所有门
		if menAna[k][0] == 0 && menAna[k][1] == 0 && menAna[k][2] == 0 && menAna[k][3] == 0 {
			allZero = true
			break
		}
	}
	fmt.Printf("allZero=%v\n", allZero)
	if allZero {
		//全0特殊牌型分析，效率可能有问题，后续需要优化
		wildOut := ExhaustionAllWildCards(noWildCards)
		for _, v := range wildOut {
			allWildOut = append(allWildOut, &common.CardInformation{
				Type:  common.CardType(v / 10).Enum(),
				Value: proto.Uint32(uint32(v % 10)),
			})
		}
		return
	}

	allWildCardsMap := make(map[int32]int32)
	for _, v := range menAna {
		wildOut := Exhaustion4Wild3NP2Cards(noWildCards, v)
		//		fmt.Printf("wildOut=%v\n", wildOut)
		for _, key := range wildOut {
			allWildCardsMap[key] = 1
		}
	}

	//	fmt.Printf("allWildCardsMap=%v\n", allWildCardsMap)

	for key, _ := range allWildCardsMap {
		allWildOut = append(allWildOut, &common.CardInformation{
			Type:  common.CardType(key / 10).Enum(),
			Value: proto.Uint32(uint32(key % 10)),
		})
	}
	return
}
func ExhaustionAllZeroWild3NP2CardsOptParallel(noWildCards []*common.CardInformation, menAna []int32) (isHu bool) {
	//排除上面已经分析出来的，每一门直接给3个进行匹配
	allExceptMatchType := [][]int{{int(common.CardType_W), 10},
		{int(common.CardType_T), 10},
		{int(common.CardType_S), 10},
		{int(common.CardType_Z), 8}}
	resChan := make(chan bool, 100) //等待癞子分析结果管道
	branches := 0                   //癞子分支数目
	//需要处理的门
	for _, v := range allExceptMatchType {
		branches++
		mt := v
		go func() {
			wildCards := make([]*common.CardInformation, 3, 3)
		outLoop:
			for x := 1; x < mt[1]; x++ {
				wildCards[0] = &common.CardInformation{
					Type:  common.CardType(mt[0]).Enum(),
					Value: proto.Uint32(uint32(x)),
				}
				for y := 1; y < mt[1]; y++ {
					wildCards[1] = &common.CardInformation{
						Type:  common.CardType(mt[0]).Enum(),
						Value: proto.Uint32(uint32(y)),
					}
					for z := 1; z < mt[1]; z++ {
						wildCards[2] = &common.CardInformation{
							Type:  common.CardType(mt[0]).Enum(),
							Value: proto.Uint32(uint32(z)),
						}
						isHu = AnaWild3NP2CardsOpt2(noWildCards, wildCards)
						if isHu {
							break outLoop
						}
					}
				}
			}
			resChan <- isHu
		}()
	}
	//等待结果
	if branches > 0 {
		for {
			isHu = <-resChan
			if isHu {
				return
			}
			branches--
			if branches <= 0 {
				break
			}
		}
	}

	return
}
func ExhaustionAllZeroWild3NP2CardsOpt(noWildCards []*common.CardInformation, menAna []int32) (isHu bool) {
	//排除上面已经分析出来的，每一门直接给3个进行匹配
	allExceptMatchType := [][]int{{int(common.CardType_W), 10},
		{int(common.CardType_T), 10},
		{int(common.CardType_S), 10},
		{int(common.CardType_Z), 8}}

	//需要处理的门
	tThen := time.Now().Nanosecond()
	for _, v := range allExceptMatchType {
		wildCards := make([]*common.CardInformation, 3, 3)
		for x := 1; x < v[1]; x++ {
			wildCards[0] = &common.CardInformation{
				Type:  common.CardType(v[0]).Enum(),
				Value: proto.Uint32(uint32(x)),
			}
			for y := 1; y < v[1]; y++ {
				wildCards[1] = &common.CardInformation{
					Type:  common.CardType(v[0]).Enum(),
					Value: proto.Uint32(uint32(y)),
				}
				for z := 1; z < v[1]; z++ {
					//					wildCards := append([]*common.CardInformation{},
					//						&common.CardInformation{
					//							Type: common.CardType(v[0]).Enum(),
					//							Value:proto.Uint32(uint32(x)),
					//						},
					//						&common.CardInformation{
					//							Type: common.CardType(v[0]).Enum(),
					//							Value:proto.Uint32(uint32(y)),
					//						},
					//						&common.CardInformation{
					//							Type: common.CardType(v[0]).Enum(),
					//							Value:proto.Uint32(uint32(z)),
					//						},
					//					)
					wildCards[2] = &common.CardInformation{
						Type:  common.CardType(v[0]).Enum(),
						Value: proto.Uint32(uint32(z)),
					}
					isHu = AnaWild3NP2CardsOpt2(noWildCards, wildCards)
					if isHu {
						return
					}
				}
			}
		}
	}
	tNow := time.Now().Nanosecond()
	fmt.Printf("06 Spent Time=%v\n", (tNow - tThen))

	return
}
func ExhaustionAllZeroWild3NP2Cards(noWildCards []*common.CardInformation, menAna []int32,
	alreadyMatch [4]int32) (wildOut []int32) {
	//排除上面已经分析出来的，每一门直接给3个进行匹配
	allExceptMatchType := [][]int{}
	for k, v := range alreadyMatch {
		if (int32(k+1) == int32(common.CardType_Z) && v < 7) ||
			(int32(k+1) != int32(common.CardType_Z) && v < 9) {
			var max int = 10
			if int32(k+1) == int32(common.CardType_Z) {
				max = 8
			}
			allExceptMatchType = append(allExceptMatchType, []int{k + 1, max})
		}
	}
	//	fmt.Printf("allExceptMatchType=%v\n", allExceptMatchType)
	//需要处理的门
	tThen := time.Now().Nanosecond()
	wildCardsMap := make(map[int32]int32)
	for _, v := range allExceptMatchType {
		for x := 1; x < v[1]; x++ {
			for y := 1; y < v[1]; y++ {
				for z := 1; z < v[1]; z++ {
					wildCards := append([]*common.CardInformation{},
						&common.CardInformation{
							Type:  common.CardType(v[0]).Enum(),
							Value: proto.Uint32(uint32(x)),
						},
						&common.CardInformation{
							Type:  common.CardType(v[0]).Enum(),
							Value: proto.Uint32(uint32(y)),
						},
						&common.CardInformation{
							Type:  common.CardType(v[0]).Enum(),
							Value: proto.Uint32(uint32(z)),
						},
					)
					out := AnaWild3NP2Cards(noWildCards, wildCards)
					if len(out) > 0 {
						for _, vc := range wildCards {
							if vc != nil {
								key := int32(vc.GetType())*10 + int32(vc.GetValue())
								wildCardsMap[key] = 1
							}
						}
					}
				}
			}
		}
	}
	tNow := time.Now().Nanosecond()
	fmt.Printf("06 Spent Time=%v\n", (tNow - tThen))
	for key, _ := range wildCardsMap {
		wildOut = append(wildOut, key)
	}
	return
}

func ExhaustionAllWildCards(noWildCards []*common.CardInformation) (
	wildOut []int32) {
	var allTypeCards = [34]*common.CardInformation{
		&common.CardInformation{Type: common.CardType_W.Enum(), Value: proto.Uint32(1)},
		&common.CardInformation{Type: common.CardType_W.Enum(), Value: proto.Uint32(2)},
		&common.CardInformation{Type: common.CardType_W.Enum(), Value: proto.Uint32(3)},
		&common.CardInformation{Type: common.CardType_W.Enum(), Value: proto.Uint32(4)},
		&common.CardInformation{Type: common.CardType_W.Enum(), Value: proto.Uint32(5)},
		&common.CardInformation{Type: common.CardType_W.Enum(), Value: proto.Uint32(6)},
		&common.CardInformation{Type: common.CardType_W.Enum(), Value: proto.Uint32(7)},
		&common.CardInformation{Type: common.CardType_W.Enum(), Value: proto.Uint32(8)},
		&common.CardInformation{Type: common.CardType_W.Enum(), Value: proto.Uint32(9)},
		&common.CardInformation{Type: common.CardType_T.Enum(), Value: proto.Uint32(1)},
		&common.CardInformation{Type: common.CardType_T.Enum(), Value: proto.Uint32(2)},
		&common.CardInformation{Type: common.CardType_T.Enum(), Value: proto.Uint32(3)},
		&common.CardInformation{Type: common.CardType_T.Enum(), Value: proto.Uint32(4)},
		&common.CardInformation{Type: common.CardType_T.Enum(), Value: proto.Uint32(5)},
		&common.CardInformation{Type: common.CardType_T.Enum(), Value: proto.Uint32(6)},
		&common.CardInformation{Type: common.CardType_T.Enum(), Value: proto.Uint32(7)},
		&common.CardInformation{Type: common.CardType_T.Enum(), Value: proto.Uint32(8)},
		&common.CardInformation{Type: common.CardType_T.Enum(), Value: proto.Uint32(9)},
		&common.CardInformation{Type: common.CardType_S.Enum(), Value: proto.Uint32(1)},
		&common.CardInformation{Type: common.CardType_S.Enum(), Value: proto.Uint32(2)},
		&common.CardInformation{Type: common.CardType_S.Enum(), Value: proto.Uint32(3)},
		&common.CardInformation{Type: common.CardType_S.Enum(), Value: proto.Uint32(4)},
		&common.CardInformation{Type: common.CardType_S.Enum(), Value: proto.Uint32(5)},
		&common.CardInformation{Type: common.CardType_S.Enum(), Value: proto.Uint32(6)},
		&common.CardInformation{Type: common.CardType_S.Enum(), Value: proto.Uint32(7)},
		&common.CardInformation{Type: common.CardType_S.Enum(), Value: proto.Uint32(8)},
		&common.CardInformation{Type: common.CardType_S.Enum(), Value: proto.Uint32(9)},
		&common.CardInformation{Type: common.CardType_Z.Enum(), Value: proto.Uint32(1)},
		&common.CardInformation{Type: common.CardType_Z.Enum(), Value: proto.Uint32(2)},
		&common.CardInformation{Type: common.CardType_Z.Enum(), Value: proto.Uint32(3)},
		&common.CardInformation{Type: common.CardType_Z.Enum(), Value: proto.Uint32(4)},
		&common.CardInformation{Type: common.CardType_Z.Enum(), Value: proto.Uint32(5)},
		&common.CardInformation{Type: common.CardType_Z.Enum(), Value: proto.Uint32(6)},
		&common.CardInformation{Type: common.CardType_Z.Enum(), Value: proto.Uint32(7)},
	}
	fmt.Printf("allZero.noWildCards=%v\n", noWildCards)
	fmt.Printf("allZero.allTypeCards=%v\n", allTypeCards)

	tThen := time.Now().Nanosecond()
	wildCardsMap := make(map[int32]int32)
	for x := 0; x < len(allTypeCards); x++ {
		for y := 0; y < len(allTypeCards); y++ {
			for z := 0; z < len(allTypeCards); z++ {
				wildCards := append([]*common.CardInformation{},
					&common.CardInformation{
						Type:  allTypeCards[x].Type,
						Value: allTypeCards[x].Value,
					},
					&common.CardInformation{
						Type:  allTypeCards[y].Type,
						Value: allTypeCards[y].Value,
					},
					&common.CardInformation{
						Type:  allTypeCards[z].Type,
						Value: allTypeCards[z].Value})
				out := AnaWild3NP2Cards(noWildCards, wildCards)
				if len(out) > 0 {
					for _, vc := range wildCards {
						if vc != nil {
							key := int32(vc.GetType())*10 + int32(vc.GetValue())
							wildCardsMap[key] = 1
						}
					}
				}
			}
		}
	}
	tNow := time.Now().Nanosecond()
	fmt.Printf("07 Spent Time=%v\n", (tNow - tThen))
	for key, _ := range wildCardsMap {
		wildOut = append(wildOut, key)
	}
	return
}

func Exhaustion4Wild3NP2CardsOptParallel(noWildCards []*common.CardInformation, menAna []int32) (isHu bool) {

	//检查癞子个数，并生成不包含癞子的手牌
	possibleCards := [][]int32{}
	//癞子变成这些牌可以成胡
	for i := 0; i < len(menAna); i++ {
		for j := 0; int32(j) < menAna[i]; j++ {
			var max int32 = 10
			if int32(i+1) == int32(common.CardType_Z) {
				max = 8
			}
			possibleCards = append(possibleCards, []int32{int32(i + 1), max})
		}
	}

	resChan := make(chan bool, 10) //等待癞子分析结果管道
	branches := 0                  //癞子分支数目
	if len(possibleCards) == 1 {
		for x := 1; int32(x) < possibleCards[0][1]; x++ {
			branches++
			valueX := x
			go func() {
				wildCards := []*common.CardInformation{
					&common.CardInformation{
						Type:  common.CardType(possibleCards[0][0]).Enum(),
						Value: proto.Uint32(uint32(valueX)),
					},
				}
				isHu = AnaWild3NP2CardsOpt2(noWildCards, wildCards)
				resChan <- isHu
			}()
		}
	} else if len(possibleCards) == 2 {
		for x := 1; int32(x) < possibleCards[0][1]; x++ {
			valueX := x
			branches++
			go func() {
				for y := 1; int32(y) < possibleCards[1][1]; y++ {
					wildCards := []*common.CardInformation{
						&common.CardInformation{
							Type:  common.CardType(possibleCards[0][0]).Enum(),
							Value: proto.Uint32(uint32(valueX)),
						},
						&common.CardInformation{
							Type:  common.CardType(possibleCards[1][0]).Enum(),
							Value: proto.Uint32(uint32(y)),
						},
					}
					isHu = AnaWild3NP2CardsOpt2(noWildCards, wildCards)
					if isHu {
						break
					}
				}
				resChan <- isHu
			}()

		}
	} else if len(possibleCards) == 3 {
		for x := 1; int32(x) < possibleCards[0][1]; x++ {
			valueX := x
			branches++
			go func() {
			outLoop:
				for y := 1; int32(y) < possibleCards[1][1]; y++ {
					for z := 1; int32(z) < possibleCards[2][1]; z++ {
						wildCards := []*common.CardInformation{
							&common.CardInformation{
								Type:  common.CardType(possibleCards[0][0]).Enum(),
								Value: proto.Uint32(uint32(valueX)),
							},
							&common.CardInformation{
								Type:  common.CardType(possibleCards[1][0]).Enum(),
								Value: proto.Uint32(uint32(y)),
							},
							&common.CardInformation{
								Type:  common.CardType(possibleCards[2][0]).Enum(),
								Value: proto.Uint32(uint32(z)),
							},
						}
						isHu = AnaWild3NP2CardsOpt2(noWildCards, wildCards)
						if isHu {
							break outLoop
						}
					}
				}

				resChan <- isHu
			}()
		}
	}
	//等待结果
	if branches > 0 {
		for {
			isHu = <-resChan
			if isHu {
				return
			}
			branches--
			if branches <= 0 {
				break
			}
		}
	}
	return
}
func Exhaustion4Wild3NP2CardsOpt(noWildCards []*common.CardInformation, menAna []int32) (isHu bool) {
	//检查癞子个数，并生成不包含癞子的手牌
	possibleCards := [][]int32{}
	//癞子变成这些牌可以成胡
	for i := 0; i < len(menAna); i++ {
		for j := 0; int32(j) < menAna[i]; j++ {
			var max int32 = 10
			if int32(i+1) == int32(common.CardType_Z) {
				max = 8
			}
			possibleCards = append(possibleCards, []int32{int32(i + 1), max})
		}
	}
	if len(possibleCards) == 1 {
		wildCards := make([]*common.CardInformation, 1, 1)
		for x := 1; int32(x) < possibleCards[0][1]; x++ {
			wildCards[0] = &common.CardInformation{
				Type:  common.CardType(possibleCards[0][0]).Enum(),
				Value: proto.Uint32(uint32(x)),
			}
			isHu = AnaWild3NP2CardsOpt2(noWildCards, wildCards)
			if isHu {
				return
			}
		}
	} else if len(possibleCards) == 2 {
		wildCards := make([]*common.CardInformation, 2, 2)
		for x := 1; int32(x) < possibleCards[0][1]; x++ {
			wildCards[0] = &common.CardInformation{
				Type:  common.CardType(possibleCards[0][0]).Enum(),
				Value: proto.Uint32(uint32(x)),
			}
			for y := 1; int32(y) < possibleCards[1][1]; y++ {

				wildCards[1] = &common.CardInformation{
					Type:  common.CardType(possibleCards[1][0]).Enum(),
					Value: proto.Uint32(uint32(y)),
				}
				isHu = AnaWild3NP2CardsOpt2(noWildCards, wildCards)
				if isHu {
					return
				}
			}
		}
	} else if len(possibleCards) == 3 {
		wildCards := make([]*common.CardInformation, 3, 3)
		count := 0
		for x := 1; int32(x) < possibleCards[0][1]; x++ {
			wildCards[0] = &common.CardInformation{
				Type:  common.CardType(possibleCards[0][0]).Enum(),
				Value: proto.Uint32(uint32(x)),
			}
			for y := 1; int32(y) < possibleCards[1][1]; y++ {
				wildCards[1] = &common.CardInformation{
					Type:  common.CardType(possibleCards[1][0]).Enum(),
					Value: proto.Uint32(uint32(y)),
				}
				for z := 1; int32(z) < possibleCards[2][1]; z++ {
					count++
					wildCards[2] = &common.CardInformation{
						Type:  common.CardType(possibleCards[2][0]).Enum(),
						Value: proto.Uint32(uint32(z)),
					}
					isHu = AnaWild3NP2CardsOpt2(noWildCards, wildCards)
					if isHu {
						return
					}
				}
			}
		}
	}

	return
}
func Exhaustion4Wild3NP2Cards(noWildCards []*common.CardInformation, menAna []int32) (
	wildCardOut []int32) {
	tThen := time.Now().Nanosecond()
	//检查癞子个数，并生成不包含癞子的手牌
	possibleCards := [][]int32{}
	//癞子变成这些牌可以成胡
	wildCardsMap := make(map[int32]int32)
	for i := 0; i < len(menAna); i++ {
		for j := 0; int32(j) < menAna[i]; j++ {
			var max int32 = 10
			if int32(i+1) == int32(common.CardType_Z) {
				max = 8
			}
			possibleCards = append(possibleCards, []int32{int32(i + 1), max})
		}
	}

	//	fmt.Printf("possibleCards=%v\n", possibleCards)
	if len(possibleCards) == 1 {
		for x := 1; int32(x) < possibleCards[0][1]; x++ {
			wildCards := append([]*common.CardInformation{}, &common.CardInformation{
				Type:  common.CardType(possibleCards[0][0]).Enum(),
				Value: proto.Uint32(uint32(x)),
			})
			out := AnaWild3NP2Cards(noWildCards, wildCards)
			if len(out) > 0 {
				for _, vc := range wildCards {
					if vc != nil {
						key := int32(vc.GetType())*10 + int32(vc.GetValue())
						wildCardsMap[key] = 1
					}
				}
			}
		}
	} else if len(possibleCards) == 2 {
		for x := 1; int32(x) < possibleCards[0][1]; x++ {
			for y := 1; int32(y) < possibleCards[1][1]; y++ {
				wildCards := append([]*common.CardInformation{},
					&common.CardInformation{
						Type:  common.CardType(possibleCards[0][0]).Enum(),
						Value: proto.Uint32(uint32(x)),
					},
					&common.CardInformation{
						Type:  common.CardType(possibleCards[1][0]).Enum(),
						Value: proto.Uint32(uint32(y)),
					},
				)
				out := AnaWild3NP2Cards(noWildCards, wildCards)
				if len(out) > 0 {
					for _, vc := range wildCards {
						if vc != nil {
							key := int32(vc.GetType())*10 + int32(vc.GetValue())
							wildCardsMap[key] = 1
						}
					}
				}
			}
		}
	} else if len(possibleCards) == 3 {
		for x := 1; int32(x) < possibleCards[0][1]; x++ {
			for y := 1; int32(y) < possibleCards[1][1]; y++ {
				for z := 1; int32(z) < possibleCards[2][1]; z++ {
					wildCards := append([]*common.CardInformation{},
						&common.CardInformation{
							Type:  common.CardType(possibleCards[0][0]).Enum(),
							Value: proto.Uint32(uint32(x)),
						},
						&common.CardInformation{
							Type:  common.CardType(possibleCards[1][0]).Enum(),
							Value: proto.Uint32(uint32(y)),
						},
						&common.CardInformation{
							Type:  common.CardType(possibleCards[2][0]).Enum(),
							Value: proto.Uint32(uint32(z)),
						},
					)
					out := AnaWild3NP2Cards(noWildCards, wildCards)
					if len(out) > 0 {
						for _, vc := range wildCards {
							if vc != nil {
								key := int32(vc.GetType())*10 + int32(vc.GetValue())
								wildCardsMap[key] = 1
							}
						}
					}
				}
			}
		}
	}
	//	fmt.Printf("wildCardsMap=%v\n", wildCardsMap)
	tThen1 := time.Now().Nanosecond()
	fmt.Printf("08 Spent Time=%v\n", (tThen1 - tThen))
	for key, _ := range wildCardsMap {
		wildCardOut = append(wildCardOut, key)
	}
	return
}

func AnaWild3NP2CardsOpt2(noWildCards []*common.CardInformation,
	wildCards []*common.CardInformation) (isHu bool) {
	fullCards := []*common.CardInformation{}
	fullCards = append(fullCards, noWildCards...)
	fullCards = append(fullCards, wildCards...)
	sort.Sort(Cards4Sort(fullCards))

	//	isHu = SplitOpt(fullCards)
	ha := &HuAnalyser001{AllPai: [4][10]int32{}}
	isHu = ha.Split002(fullCards)
	return
}

func AnaWild3NP2CardsOpt(noWildCards []*common.CardInformation,
	wildCards []*common.CardInformation) (isHu bool) {

	fullCards := sortInsertCards(noWildCards, wildCards)

	factorN := ((len(fullCards) + 1) / 3) - 1
	//分析手牌

	AnaRes := Split(fullCards)
	//是否满足3n+2

	for _, v := range AnaRes {
		if len(v.GetLeft()) == 0 && len(v.GetTwos()) == 1 && len(v.GetSeqs())+len(v.GetThrees()) == factorN {
			//其它胡牌类型
			isHu = true
			break
		}
	}

	return
}

func AnaWild3NP2Cards(noWildCards []*common.CardInformation,
	wildCards []*common.CardInformation) (AnaOut []*common.AnalyzeResult) {
	//	fmt.Printf("\nnoWildCards=%v\n", noWildCards)
	//	fmt.Printf("wildCards=%v\n", wildCards)
	tThen := time.Now().Nanosecond()
	fullCards := sortInsertCards(noWildCards, wildCards)
	tThen1 := time.Now().Nanosecond()
	fmt.Printf("0901 Spent Time=%v\n", (tThen1 - tThen))
	factorN := ((len(fullCards) + 1) / 3) - 1
	//分析手牌
	t1 := time.Now().Nanosecond()
	AnaRes := Split(fullCards)
	//是否满足3n+2
	t2 := time.Now().Nanosecond()
	fmt.Printf("0902 Spent Time=%v\n", (t2 - t1))
	//	fmt.Printf("fullCards=%v\n", fullCards)
	for _, v := range AnaRes {
		//		fmt.Printf("AnaRes=%v\n", v)
		if len(v.GetLeft()) == 0 && len(v.GetTwos()) == 1 && len(v.GetSeqs())+len(v.GetThrees()) == factorN {
			//其它胡牌类型
			AnaOut = append(AnaOut, v)
		}
	}
	t3 := time.Now().Nanosecond()
	fmt.Printf("0903 Spent Time=%v\n", (t3 - t2))
	return
}

func sortInsertCards(noWildCards []*common.CardInformation,
	wildCards []*common.CardInformation) (out []*common.CardInformation) {
	//拷贝一次
	copyCards := []*common.CardInformation{}
	for _, v := range noWildCards {
		copyCards = append(copyCards, &common.CardInformation{
			Type:  v.Type,
			Value: v.Value,
		})
	}
	for _, v := range wildCards {
		if v != nil {
			hasInsert := false
			for k := 0; k < len(copyCards); k++ {
				inner := copyCards[k]
				if inner != nil {
					if inner.GetType() != v.GetType() {
						if inner.GetType() > v.GetType() {
							rear := append([]*common.CardInformation{}, copyCards[k:]...)
							copyCards = append(copyCards[:k], v)
							copyCards = append(copyCards, rear...)
							hasInsert = true
							break
						}
					} else {
						if inner.GetValue() >= v.GetValue() {
							rear := append([]*common.CardInformation{}, copyCards[k:]...)
							copyCards = append(copyCards[:k], v)
							copyCards = append(copyCards, rear...)
							hasInsert = true
							break
						}
					}
				}
			}
			//没中间插，就插在最后
			if !hasInsert {
				copyCards = append(copyCards, v)
			}
		}
	}
	out = append(out, copyCards...)
	return
}

func is7Dui(handCards []*common.CardInformation) (r bool) {
	if len(handCards) == 14 {
		duiCount := 0
		for i := 0; i < len(handCards); {
			if CardEq(handCards[i], handCards[i+1]) {
				duiCount++
				i = i + 2
			} else {
				break
			}
		}
		if duiCount == 7 {
			r = true
		}
	}

	return
}

type HuAnalyser001 struct {
	AllPai [4][10]int32
}

/*另一种递归折牌算法*/
func (ha *HuAnalyser001) Split001(ts []*common.CardInformation) (isHu bool) {
	if is7Dui(ts) {
		isHu = true
		return
	} else if ha.recursive3np2(ts) {
		isHu = true
		return
	}
	return
}

func (ha *HuAnalyser001) Split002(ts []*common.CardInformation) (isHu bool) {
	if ha.recursive3np2(ts) {
		isHu = true
		return
	}
	return
}

func (ha *HuAnalyser001) recursive3np2(ts []*common.CardInformation) (r bool) {
	ha.init(ts)
	//	fmt.Printf("allpay=%v\n", ha.AllPai)
	r = ha.firstLevel()
	return
}
func (ha *HuAnalyser001) init(ts []*common.CardInformation) {
	for i := 0; i < len(ts); i++ {
		t := ts[i].GetType()
		v := ts[i].GetValue()
		if (common.CardType_W > t || t > common.CardType_S || 1 > v || v > 9) &&
			(t != common.CardType_Z || 1 > v || v > 7) {
			return
		}
		ha.AllPai[t-1][v]++
		ha.AllPai[t-1][0]++
	}
}

func (ha *HuAnalyser001) firstLevel() bool {
	yuShu := int32(0)
	jiangPos := 0
	jiangExisted := false
	x := 0
	for ; x < 4; x++ {
		yuShu = ha.AllPai[x][0] % 3
		if yuShu == 1 {
			break
		}
		if yuShu == 2 {
			if jiangExisted {
				break
			}
			jiangPos = x
			jiangExisted = true
		}
	}

	if x < 4 {
		return false
	}

	k := 0
	for ; k < 4; k++ {
		if k != jiangPos {
			if !ha.split3np2(int32(k)) {
				break
			}
		}
	}
	if k < 4 {
		return false
	}

	s := false
	for z := 1; z < 10; z++ {
		if ha.AllPai[jiangPos][z] >= 2 {
			ha.AllPai[jiangPos][z] -= 2
			ha.AllPai[jiangPos][0] -= 2
			s = ha.split3np2(int32(jiangPos))
			ha.AllPai[jiangPos][z] += 2
			ha.AllPai[jiangPos][0] += 2
			if s {
				break
			}
		}
	}
	return s
}
func (ha *HuAnalyser001) split3np2(ctype int32) bool {
	if ha.AllPai[ctype][0] == 0 {
		return true
	}
	i := 1
	for ; i < 10; i++ {
		if ha.AllPai[ctype][i] != 0 {
			break
		}
	}

	result := false
	if ha.AllPai[ctype][i] >= 3 {
		ha.AllPai[ctype][i] -= 3
		ha.AllPai[ctype][0] -= 3
		result = ha.split3np2(ctype)
		ha.AllPai[ctype][i] += 3
		ha.AllPai[ctype][0] += 3
		return result
	}

	if (ctype != 3) && (i < 8) &&
		(ha.AllPai[ctype][i+1] > 0) &&
		(ha.AllPai[ctype][i+2] > 0) {
		ha.AllPai[ctype][i]--
		ha.AllPai[ctype][i+1]--
		ha.AllPai[ctype][i+2]--
		ha.AllPai[ctype][0] -= 3
		result = ha.split3np2(ctype)
		ha.AllPai[ctype][i]++
		ha.AllPai[ctype][i+1]++
		ha.AllPai[ctype][i+2]++
		ha.AllPai[ctype][0] += 3
		return result
	}
	return false
}
