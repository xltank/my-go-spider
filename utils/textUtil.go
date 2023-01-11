package utils

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"my-go-spider/model"
	"net/http"
	"strings"
	"time"

	"github.com/go-dedup/simhash/simhashCJK"
	"github.com/google/uuid"
	"github.com/lukechampine/fastxor"
	"github.com/seefan/gossdb/v2"
)

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
  Split simHash(base 16) to 4 parts, save to cache:
	1, simHashA_part1 -> [ts_simHashA, ts_simHashB, ...]
	2, simHashA -> relA
*/
func CacheSimHash(simHashStr string, publishedAt time.Time, rel string) {
	cache, err := gossdb.NewClient()
	if err != nil {
		panic("new client of ssdb error:" + err.Error())
	}
	defer cache.Close()

	for i := 0; i < len(simHashStr); i += 4 {
		s := simHashStr[i : i+4]
		key := simHashPartKey + ":" + s
		fmt.Printf("cache.Set %s %s\n", key, rel)
		err = cache.ZSet(key, string(publishedAt.Unix())+"_"+simHashStr, publishedAt.UnixMilli())
		if err != nil {
			fmt.Printf("cache.Set error: %s, %s === %s\n", key, rel, err.Error())
		}
	}
}

/*

  1, get simHashs by simHash parts from cache;
	2, check simHashs, pick the one by order: best match, latest publishedAt;
	3, remember those staled items and delete them later by multi_zdel();
	4, get rel by matched simHash;
	5, if no suitable simHash, call CacheSimHash();
*/
func GetRelBySimHash(simHashStr string, publishedAt time.Time) (rel string) {
	rel = ""
	sourceBytes, _ := hex.DecodeString(simHashStr)

	cache, err := gossdb.NewClient()
	if err != nil {
		panic("new client of ssdb error:" + err.Error())
	}
	defer cache.Close()

	simHashList := []string{}
	for i := 0; i < len(simHashStr); i += 4 {
		key := simHashStr[i : i+4]
		hashList, err := cache.ZKeys(key, 0, 100)
		if err != nil {
			fmt.Printf("ssdb get zset error, key: %s, error: %s", key, err.Error())
		}
		simHashList = append(simHashList, hashList...)

		for v := range hashList {
			bytes, _ := hex.DecodeString(v)
			var r []byte
			n := fastxor.Bytes(r, sourceBytes, bytes)
			fmt.Printf("xor, %s, %s, n: %v", sourceBytes, bytes, n)
		}

		// if rel != "" && rel != v.String() {
		// 	fmt.Println("weird rel cache:", i, key, rel, v.String())
		// }
		// rel = v.String()
	}
	if rel == "" {
		rel = uuid.New().String()
		fmt.Println("GetRelBySimHash, not found, make a new one:", rel)
		CacheSimHash(simHashStr, publishedAt, rel)
	}
	return rel
}
