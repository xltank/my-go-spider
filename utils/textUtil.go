package utils

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"my-go-spider/db"
	"my-go-spider/model"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/go-dedup/simhash/simhashCJK"
	"github.com/go-redis/redis/v9"
	"github.com/lukechampine/fastxor"
)

const HashKeyExpire = 86400 * 3

func ParseText(text string) []model.Token {
	t := fmt.Sprintf(`{"analyzer": "my_hanlp_analyzer", "text": "%s"}`, text)
	resp, err := http.Post(`http://localhost:9200/hanlp-1/_analyze`, `application/json`, strings.NewReader(t))
	if err != nil {
		fmt.Println(`ParseText Error: `, err.Error())
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(`ParseText, read resp.Body error: `, err.Error())
	}
	r := model.AnalyzedResult{}
	err = json.Unmarshal(body, &r)
	if err != nil {
		fmt.Println(`Unmarshal analyze result Error: `, err.Error())
	}
	return r.Tokens
}

/*
  Generate simHash in base16 mode
*/
func GetSimHash(text string) (hash string) {
	sh := simhashCJK.NewSimhash()
	rawHash := sh.GetSimhash(sh.NewCJKWordFeatureSet([]byte(text)))
	fmt.Printf("=== %s %b %d %x \n", text, rawHash, rawHash, rawHash)
	return fmt.Sprintf("%x", rawHash)
}

const simHashPartKey = "spider:local:simHashPart"
const simHasnRelKey = "spider:local:simHashRel"

/*
  Split simHash(base 16) to 4 parts, save to cache; If rel == "", create one;
	1, simHashA_part1 -> [ts_simHashA, ts_simHashB, ...]
	2, simHashA -> relA
*/
func CacheSimHash(simHashStr string, publishedAt time.Time, rel string) {
	for i := 0; i < len(simHashStr); i += 4 {
		s := simHashStr[i : i+4]
		key := simHashPartKey + ":" + s
		fmt.Printf("cache.Set %s %s\n", key, rel)

		var ctx = context.Background()
		err := db.Redis.ZAdd(ctx, key, redis.Z{Score: float64(publishedAt.UnixMilli()), Member: simHashStr}).Err()
		if err != nil {
			fmt.Printf("Set %s error: %s, %s === %s\n", simHashPartKey, key, rel, err.Error())
		}
		err = db.Redis.Expire(ctx, key, HashKeyExpire).Err()
		if err != nil {
			fmt.Printf("Set %s error: %s, %s === %s\n", simHashPartKey, key, rel, err.Error())
		}

		ctx = context.Background()
		err = db.Redis.SetXX(ctx, key, rel, HashKeyExpire).Err()
		if err != nil {
			fmt.Printf("Set %s error: %s, %s === %s\n", simHashPartKey, key, rel, err.Error())
		}
	}
}

func getHashByKey(c chan []string, key string) {
	ctx := context.Background()
	result, err := db.Redis.ZRevRange(ctx, key, 0, 100).Result()
	if err != nil {
		fmt.Printf("ssdb get zset error, key: %s, error: %s", key, err.Error())
	}
	c <- result
}

/*
  1, get simHash list by 4 simHash parts from cache;
	2, check simHashs, pick the one by order: best match; // , latest publishedAt;
	3, delete staled items by Redis.ZREMRANGEBYSCORE;
	4, get rel by matched simHash;
	5, if no suitable simHash, call CacheSimHash();
*/
func GetRelBySimHash(simHashStr string, publishedAt time.Time) (rel string) {
	rel = ""
	sourceBytes, _ := hex.DecodeString(simHashStr)

	simHashList := []string{}
	c := make(chan []string)
	for i := 0; i < len(simHashStr); i += 4 {
		key := simHashStr[i : i+4]
		go getHashByKey(c, key)
	}
	for list := range c {
		simHashList = append(simHashList, list...)
	}

	targetHashList := []model.HashGap{}
	for _, v := range simHashList {
		bytes, _ := hex.DecodeString(v)
		var r []byte
		xorBytes := fastxor.Bytes(r, sourceBytes, bytes)
		gap := countBit1(uint64(xorBytes))
		fmt.Printf("xor, %s, %s, n: %v", sourceBytes, bytes, gap)
		if gap <= 3 {
			targetHashList = append(targetHashList, model.HashGap{gap, v})
		}
	}
	fmt.Println("targetHashList: ", targetHashList)

	var bestHash string
	if len(targetHashList) > 0 {
		bestHashGap := getBestHashGap(targetHashList)
		bestHash = bestHashGap.Hash
	}

	if bestHash != "" {

	}

	// if rel == "" {
	// 	rel = uuid.New().String()
	// 	fmt.Println("GetRelBySimHash, not found, make a new one:", rel)
	// 	CacheSimHash(simHashStr, publishedAt, rel)
	// }
	// return rel
}

func countBit1(n uint64) (count uint64) {
	for n != 0 {
		count += n & uint64(1)
		n >>= 1
	}
	return
}

func getBestHashGap(list []model.HashGap) model.HashGap {
	sort.Slice(list, func(i, j int) bool {
		return list[i].Count < list[j].Count
	})
	return list[0]
}
