package questions

import "github.com/ricky40043/guess-who-game/internal/game"

var Bank = []game.Question{
	{ID: 1, Text: "小時候最討厭上的課是什麼？", Category: "童年回憶"},
	{ID: 2, Text: "小時候最常被家人罵的原因是什麼？", Category: "童年回憶"},
	{ID: 3, Text: "小時候最常看的卡通是哪一部？", Category: "童年回憶"},
	{ID: 4, Text: "小時候最想當什麼職業？", Category: "童年回憶"},
	{ID: 5, Text: "小時候最怕的東西是什麼？", Category: "童年回憶"},
	{ID: 6, Text: "小時候最常玩的遊戲是什麼？", Category: "童年回憶"},
	{ID: 7, Text: "小時候曾經相信過什麼奇怪的事情？", Category: "童年回憶"},
	{ID: 8, Text: "小時候最喜歡收集什麼東西？", Category: "童年回憶"},
	{ID: 9, Text: "小時候做過最調皮的事情是什麼？", Category: "童年回憶"},
	{ID: 10, Text: "小時候最常吃的早餐是什麼？", Category: "童年回憶"},
	{ID: 11, Text: "學生時代最喜歡的科目是什麼？", Category: "學校生活"},
	{ID: 12, Text: "學生時代最害怕哪一種老師？", Category: "學校生活"},
	{ID: 13, Text: "曾經考過最慘的是哪一科？", Category: "學校生活"},
	{ID: 14, Text: "上課時最常偷偷做什麼？", Category: "學校生活"},
	{ID: 15, Text: "曾經因為什麼原因被老師點名？", Category: "學校生活"},
	{ID: 16, Text: "學生時代參加過什麼社團？", Category: "學校生活"},
	{ID: 17, Text: "曾經最丟臉的一次上台經驗是什麼？", Category: "學校生活"},
	{ID: 18, Text: "學校午餐最不想看到哪一道菜？", Category: "學校生活"},
	{ID: 19, Text: "最喜歡的甜點是什麼？", Category: "飲食習慣"},
	{ID: 20, Text: "最討厭的蔬菜是什麼？", Category: "飲食習慣"},
	{ID: 21, Text: "吃火鍋一定會拿的食材是什麼？", Category: "飲食習慣"},
	{ID: 22, Text: "最不能接受哪一種披薩配料？", Category: "飲食習慣"},
	{ID: 23, Text: "最常點的飲料是什麼？", Category: "飲食習慣"},
	{ID: 24, Text: "吃鹹酥雞一定會點什麼？", Category: "飲食習慣"},
	{ID: 25, Text: "最不能理解大家為什麼喜歡哪種食物？", Category: "飲食習慣"},
	{ID: 26, Text: "起床後第一件事通常做什麼？", Category: "個性與日常"},
	{ID: 27, Text: "洗澡時最常做什麼奇怪的事？", Category: "個性與日常"},
	{ID: 28, Text: "曾經最久幾天沒有整理房間？", Category: "個性與日常"},
	{ID: 29, Text: "心情不好時最常做什麼？", Category: "個性與日常"},
	{ID: 30, Text: "曾經為了省錢做過什麼事情？", Category: "個性與日常"},
	{ID: 31, Text: "最容易因為什麼小事生氣？", Category: "個性與日常"},
	{ID: 32, Text: "從小到大暗戀過幾個人？", Category: "感情與尷尬往事"},
	{ID: 33, Text: "第一次喜歡上一個人是幾歲？", Category: "感情與尷尬往事"},
	{ID: 34, Text: "曾經用過最尷尬的告白方式是什麼？", Category: "感情與尷尬往事"},
	{ID: 35, Text: "曾經為了喜歡的人做過什麼傻事？", Category: "感情與尷尬往事"},
	{ID: 36, Text: "最後一次尿床大約是幾歲？", Category: "感情與尷尬往事"},
}

func MapByID() map[int]game.Question {
	result := make(map[int]game.Question, len(Bank))
	for _, question := range Bank {
		result[question.ID] = question
	}
	return result
}
