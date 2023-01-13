package utils

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"my-go-spider/db"
	"my-go-spider/model"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-dedup/simhash/simhashCJK"
	"github.com/go-redis/redis/v9"
	"github.com/google/uuid"
	"github.com/lukechampine/fastxor"
)

const HashKeyExpire = 86400 * 3 * time.Second

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
	Cache logic:
	1, Saving:
  	1, Split simHash(base 16) to 4 parts, save to cache; If rel == "", create one;
		2, zadd simHashA_part1 ts simHashA;
		3, expire simHashA_part1 86400*3;
		4, set simHashA -> relA 86400*3 ex 86400*3;
*/

/*
 */
func CacheSimHash(simHashStr string, publishedAt time.Time, rel string) {
	for i := 0; i < len(simHashStr); i += 4 {
		s := simHashStr[i : i+4]
		partKey := simHashPartKey + ":" + s
		fmt.Printf("cache.Set %s %s\n", partKey, rel)

		var ctx = context.Background()
		err := db.Redis.ZAdd(ctx, partKey, redis.Z{Score: float64(publishedAt.UnixMilli()), Member: simHashStr}).Err()
		if err != nil {
			fmt.Printf("Set %s error: %s, %s === %s\n", simHashPartKey, partKey, rel, err.Error())
		}
		err = db.Redis.Expire(ctx, partKey, HashKeyExpire).Err()
		if err != nil {
			fmt.Printf("Set %s error: %s, %s === %s\n", simHashPartKey, partKey, rel, err.Error())
		}

		ctx = context.Background()
		hashRelKey := simHasnRelKey + ":" + simHashStr
		err = db.Redis.SetEx(ctx, hashRelKey, rel, HashKeyExpire).Err()
		if err != nil {
			fmt.Printf("Set %s error: %s, %s === %s\n", simHasnRelKey, hashRelKey, rel, err.Error())
		}
	}
}

func getHashByKey(c chan []string, key string) {
	ctx := context.Background()
	result, err := db.Redis.ZRevRange(ctx, key, 0, 10).Result()
	if err != nil {
		fmt.Printf("ssdb get zset error, key: %s, error: %s", key, err.Error())
	}
	fmt.Println("getHashByKey:", key, result)
	c <- result
}

/*
Cache logic:
	2, Matching:
		1, get top 10 latest simHash list of 4 parts by ZRANGE simHashA_part1 0 10 REV;
		2, get suitable one with xor threshold 3;
		3, if result == nil, generate new rel and do Saving, return rel;
		4, if result != nil, find rel by result hash and return it;
		5, refresh 4 parts keys ttl: expire simHashA_part1 86400*3;
		6, clear sorted set of 4 parts: ZREMRANGEBYSCORE simHashA_part1 0 (now - 3 days), which means removing set item whose score is less than now - 3 days;
*/
func GetRelBySimHash(simHashStr string, publishedAt time.Time) string {
	sourceBytes, _ := hex.DecodeString(simHashStr)

	simHashList := []string{}
	c := make(chan []string)
	var wg sync.WaitGroup

	for i := 0; i+4 <= len(simHashStr); i += 4 {
		partKey := simHashPartKey + ":" + simHashStr[i:i+4]
		wg.Add(1)
		go func() {
			defer wg.Done()
			getHashByKey(c, partKey)
		}()
	}

	go func() {
		wg.Wait()
		close(c)
	}()

	for list := range c {
		simHashList = append(simHashList, list...)
	}
	fmt.Println("simHashList: ", simHashList)

	targetHashList := []model.HashGap{}
	for _, v := range simHashList {
		bytes, _ := hex.DecodeString(v)
		r := make([]byte, 8)
		r1 := r[0:]
		fastxor.Bytes(r1, sourceBytes, bytes)
		gap := countBit1(binary.BigEndian.Uint64(r))
		fmt.Printf("xor\n%08b\n%08b\n%08b\ngap: %v\n", sourceBytes, bytes, r1, gap)
		if gap <= 3 {
			targetHashList = append(targetHashList, model.HashGap{Count: gap, Hash: v})
		}
	}
	fmt.Println("targetHashList: ", targetHashList)

	var bestHash string
	if len(targetHashList) > 0 {
		bestHashGap := getBestHashGap(targetHashList)
		bestHash = bestHashGap.Hash
	}

	if bestHash != "" {
		hashRelKey := simHasnRelKey + ":" + simHashStr
		ctx := context.Background()
		rel, err := db.Redis.Get(ctx, hashRelKey).Result()
		if err != nil {
			fmt.Printf("getting %s of %s error: \n%s", simHasnRelKey, hashRelKey, err.Error())
		}
		fmt.Println("best hash:", bestHash, ", rel:", rel)
		return rel
	} else {
		rel := uuid.New().String()
		CacheSimHash(simHashStr, publishedAt, rel)
		fmt.Println("GetRelBySimHash, not found, make a new one and cache it:", simHashStr, publishedAt, rel)
		return rel
	}
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
