package trailers

import "testing"

func TestFencedExamples(t *testing.T) {
	for _, body := range []string{"~~~\nBrief: demo/01\n~~~\n", "````md\n```\nBrief: demo/01\n````\n", "~~~md\n```\nBrief: demo/01\n~~~\n", "```md\n~~~\nBrief: demo/01\n```\n", "~~~\nBrief: demo/01\n"} {
		if ts := Scan([]byte(body)); len(ts) != 0 {
			t.Fatalf("example became work: %q: %v", body, ts)
		}
	}
	ts, e := ParseStrict([]byte("~~~\nBrief: demo/99\n~~~\nBrief: demo/01\r\n"))
	if e != nil || len(ts) != 1 || ts[0].Value != "demo/01" || ts[0].Line != 4 {
		t.Fatal(ts, e)
	}
}
func TestTrailerProfiles(t *testing.T) {
	body := []byte("Authors: demo/01\nBrief: demo/02\n")
	if len(Scan(body)) != 2 {
		t.Fatal("historical facts lost")
	}
	if _, e := ParseStrict(body); e == nil {
		t.Fatal("new mixed work link admitted")
	}
	if ts, e := ParseStrict([]byte("Authors: demo/01\n")); e != nil || len(ts) != 1 || ts[0].Kind != TrailerAuthors {
		t.Fatal(ts, e)
	}
}
