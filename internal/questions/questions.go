package questions

import "github.com/ricky40043/guess-who-game/internal/game"

var Bank = []game.Question{
	{ID: 1, Text: "小時候最討厭上的課是什麼？", Category: "童年回憶"},
	{ID: 2, Text: "最後一次尿床是幾歲？", Category: "尷尬趣事"},
	{ID: 3, Text: "從小到大暗戀過幾個人？", Category: "感情與喜好"},
	{ID: 4, Text: "喜歡的對象最重要的三個條件？", Category: "感情與喜好"},
	{ID: 5, Text: "最喜歡的甜點是什麼？", Category: "飲食習慣"},
	{ID: 6, Text: "最討厭的蔬菜是什麼？", Category: "飲食習慣"},
	{ID: 7, Text: "學生時代最常被老師提醒什麼？", Category: "童年回憶"},
	{ID: 8, Text: "最想刪掉的一段黑歷史是什麼？", Category: "尷尬趣事"},
	{ID: 9, Text: "你最常遲到的理由是什麼？", Category: "生活習慣"},
	{ID: 10, Text: "如果只能吃一種宵夜，你會選什麼？", Category: "飲食習慣"},
	{ID: 11, Text: "最不敢看的電影類型？", Category: "感情與喜好"},
	{ID: 12, Text: "最想重新體驗哪一個年紀？", Category: "童年回憶"},
	{ID: 13, Text: "你手機裡最多的是哪一類照片？", Category: "生活習慣"},
	{ID: 14, Text: "最容易讓你生氣的小事？", Category: "個性與生活"},
	{ID: 15, Text: "你最常用的口頭禪是什麼？", Category: "個性與生活"},
	{ID: 16, Text: "第一次打工做的是什麼？", Category: "童年回憶"},
	{ID: 17, Text: "最想學會但一直沒學的技能？", Category: "未來想像"},
	{ID: 18, Text: "旅行時最不能少的東西？", Category: "生活習慣"},
	{ID: 19, Text: "最愛唱的 KTV 必點歌？", Category: "感情與喜好"},
	{ID: 20, Text: "你覺得自己最像哪一種動物？", Category: "個性與生活"},
	{ID: 21, Text: "如果中樂透第一件事做什麼？", Category: "未來想像"},
	{ID: 22, Text: "最不能接受別人碰你的什麼東西？", Category: "生活習慣"},
	{ID: 23, Text: "最常被朋友吐槽的缺點？", Category: "個性與生活"},
	{ID: 24, Text: "吃火鍋一定要點什麼？", Category: "飲食習慣"},
	{ID: 25, Text: "最喜歡哪一個季節？為什麼？", Category: "感情與喜好"},
	{ID: 26, Text: "小時候最想從事什麼職業？", Category: "童年回憶"},
	{ID: 27, Text: "最荒謬的一次迷路經驗？", Category: "尷尬趣事"},
	{ID: 28, Text: "你洗澡時最常做什麼額外的事？", Category: "生活習慣"},
	{ID: 29, Text: "如果明天不用上班，你今晚會做什麼？", Category: "未來想像"},
	{ID: 30, Text: "最想住在哪一個城市或國家？", Category: "未來想像"},
	{ID: 31, Text: "最愛的早餐組合？", Category: "飲食習慣"},
	{ID: 32, Text: "你最怕哪一種昆蟲？", Category: "感情與喜好"},
	{ID: 33, Text: "曾經為了什麼事情裝病？", Category: "尷尬趣事"},
	{ID: 34, Text: "最常在哪件事上選擇困難？", Category: "個性與生活"},
	{ID: 35, Text: "最想擁有哪一個超能力？", Category: "未來想像"},
	{ID: 36, Text: "如果可以改名字，你會改成什麼？", Category: "未來想像"},
	{ID: 37, Text: "最難戒掉的習慣？", Category: "生活習慣"},
	{ID: 38, Text: "你最會做的一道料理？", Category: "飲食習慣"},
	{ID: 39, Text: "最喜歡收到哪一類禮物？", Category: "感情與喜好"},
	{ID: 40, Text: "小時候最常看的卡通？", Category: "童年回憶"},
	{ID: 41, Text: "最丟臉的一次叫錯人經驗？", Category: "尷尬趣事"},
	{ID: 42, Text: "如果被困在無人島，只能帶一樣東西？", Category: "未來想像"},
	{ID: 43, Text: "你最常在幾點睡覺？", Category: "生活習慣"},
	{ID: 44, Text: "最喜歡的冰淇淋口味？", Category: "飲食習慣"},
	{ID: 45, Text: "別人做什麼會讓你好感大增？", Category: "感情與喜好"},
	{ID: 46, Text: "你覺得自己是早起型還是夜貓型？", Category: "個性與生活"},
	{ID: 47, Text: "人生中最衝動買過什麼？", Category: "尷尬趣事"},
	{ID: 48, Text: "如果可以回到學生時代一天，你會做什麼？", Category: "童年回憶"},
	{ID: 49, Text: "最想挑戰的一件瘋狂事情？", Category: "未來想像"},
	{ID: 50, Text: "你覺得朋友最容易用哪三個字形容你？", Category: "個性與生活"},
}

func MapByID() map[int]game.Question {
	result := make(map[int]game.Question, len(Bank))
	for _, question := range Bank {
		result[question.ID] = question
	}
	return result
}
