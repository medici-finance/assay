package perfrec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// regression: #4360
func TestV4ExternalConsumer(t *testing.T) {
	self, e := filepath.Abs(".")
	if e != nil {
		t.Fatal(e)
	}
	consumer := consumerModule(t, self, "")
	main := `package main
import("encoding/json";"fmt";"strings";"github.com/medici-finance/assay/perfrec")
func main(){
 c:=perfrec.ResolveContext(perfrec.ContextInput{Profile:perfrec.Value{Value:"tool",Present:true},LookupEnv:func(k string)(string,bool){return "1",k==perfrec.EnvVar}})
 r:=perfrec.BeginSelected(perfrec.Probe{},perfrec.Probe{}.Mark(),nil,c).Finish("consumer","run","",0,"")
 r.Install,_=perfrec.NewID();v4,e:=perfrec.ToV4(r);if e!=nil{panic(e)};raw,e:=json.Marshal(v4);if e!=nil{panic(e)}
 raw=[]byte(strings.Replace(string(raw),"\"extensions\":{","\"extensions\":{\"future\":{\"version\":2,\"data\":{\"private-name\":\"secret-canary\"}},",1))
 got,e:=perfrec.Decode(raw);if e!=nil{panic(e)};out,e:=json.Marshal(got);if e!=nil||strings.Contains(string(out),"secret-canary"){panic("raw payload escape")}
 s,e:=perfrec.ReduceV4([]perfrec.Record{got},perfrec.ProfileTool,"tool",perfrec.V3Filter{});if e!=nil||s.Records!=1||s.UnsupportedExtensions!=1{panic("reduction failed")};fmt.Print("v4 external consumer PASS")
}`
	if e = os.WriteFile(filepath.Join(consumer, "main.go"), []byte(main), 0600); e != nil {
		t.Fatal(e)
	}
	out, e := goCmd(t, consumer, "run", ".")
	if e != nil || strings.TrimSpace(string(out)) != "v4 external consumer PASS" {
		t.Fatalf("%v %s", e, out)
	}
	if bad := closureViolations(t, consumer); len(bad) > 0 {
		t.Fatal(bad)
	}
}
