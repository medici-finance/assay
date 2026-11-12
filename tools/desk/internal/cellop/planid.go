package cellop

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"math/big"
	"os"
)

// A plan id is two words joined by a hyphen, an adjective then a noun ("amber-falcon"), so an
// operator can read it off one screen and type it into another. Both lists are fixed and compiled
// in; every word is 3 to 8 lowercase letters, no word is in both lists, and every pairing is
// meant to be harmless to read aloud: no people, body parts, slang, colours that name people, or
// words with a second meaning. The id is a name, not a credential: the typed confirm code is what
// an apply checks, and the id only finds the plan.
//
// A plan stored before two-word ids, under a 24-hex id, still loads and applies (idRE accepts both).

var idAdjectives = [...]string{
	"able", "agile", "airy", "amber", "ample", "apt", "astral", "azure", "balmy", "bold",
	"brave", "breezy", "bright", "brisk", "calm", "candid", "cheery", "chipper", "civil", "clear",
	"clever", "cobalt", "cordial", "cosmic", "coral", "cozy", "crisp", "curious", "dapper", "daring",
	"deft", "dewy", "eager", "early", "earnest", "elegant", "epic", "exact", "fabled", "fair",
	"festive", "fleet", "fluent", "frank", "fresh", "frosty", "gallant", "genial", "gentle", "glad",
	"glossy", "graceful", "grand", "happy", "hardy", "hearty", "helpful", "honest", "humble", "jade",
	"jaunty", "jolly", "jovial", "joyful", "keen", "kind", "kindly", "limber", "lively", "lofty",
	"loyal", "lucid", "lucky", "lunar", "mellow", "merry", "mighty", "mild", "misty", "modest",
	"mossy", "neat", "nifty", "nimble", "noble", "novel", "oaken", "opal", "orderly", "patient",
	"peppy", "placid", "plucky", "plush", "poised", "polar", "polite", "prime", "prompt", "proud",
	"quaint", "quick", "quiet", "radiant", "rapid", "ready", "regal", "robust", "royal", "rustic",
	"sage", "scenic", "serene", "sharp", "shiny", "silver", "sincere", "sleek", "smart", "snowy",
	"snug", "solar", "solid", "sonic", "spry", "stable", "steady", "stellar", "stoic", "sturdy",
	"sublime", "sunlit", "sunny", "swift", "teal", "thrifty", "tidy", "timely", "topaz", "tranquil",
	"true", "trusty", "upbeat", "valid", "vast", "velvet", "verdant", "vernal", "vital", "vivid",
	"warm", "wise", "wistful", "witty", "zesty", "zippy",
}

var idNouns = [...]string{
	"falcon", "heron", "otter", "badger", "lynx", "marten", "osprey", "kestrel", "plover", "puffin",
	"robin", "wren", "finch", "sparrow", "starling", "lark", "linnet", "egret", "crane", "ibis",
	"pelican", "gannet", "petrel", "tern", "curlew", "dunlin", "avocet", "warbler", "oriole", "magpie",
	"jay", "raven", "hawk", "eagle", "condor", "owl", "kite", "harrier", "bison", "moose",
	"elk", "ibex", "gazelle", "impala", "oryx", "okapi", "zebra", "giraffe", "panda", "koala",
	"wombat", "lemur", "seal", "walrus", "dolphin", "orca", "narwhal", "beluga", "salmon", "trout",
	"pike", "minnow", "marlin", "tuna", "herring", "koi", "gecko", "iguana", "newt", "frog",
	"tortoise", "turtle", "hare", "fox", "bear", "lion", "tiger", "cheetah", "jaguar", "puma",
	"ocelot", "serval", "caracal", "meerkat", "stoat", "mink", "alpaca", "llama", "camel", "yak",
	"aspen", "birch", "cedar", "cypress", "elm", "fir", "hazel", "juniper", "larch", "laurel",
	"linden", "maple", "oak", "olive", "pine", "poplar", "rowan", "spruce", "willow", "yew",
	"alder", "acacia", "baobab", "banyan", "magnolia", "sequoia", "sycamore", "fern", "clover", "thistle",
	"lotus", "orchid", "tulip", "daisy", "poppy", "iris", "aster", "dahlia", "lilac", "heather",
	"ivy", "brook", "canyon", "delta", "dune", "fjord", "glacier", "glade", "grove", "harbor",
	"lagoon", "meadow", "mesa", "prairie", "reef", "ridge", "river", "summit", "tundra", "valley",
	"comet", "galaxy", "nebula", "quasar", "aurora", "meteor", "orbit", "planet", "pulsar", "zenith",
	"horizon",
}

// maxDraws bounds how many ids SaveNew draws before it gives up: a draw is retried only when its
// id is already in the store, so running out means the store holds a large share of every pair.
const maxDraws = 64

// drawID draws one id from the word lists. Tests replace it to force collisions.
var drawID = func() (string, error) {
	a, err := pick(len(idAdjectives))
	if err != nil {
		return "", err
	}
	n, err := pick(len(idNouns))
	if err != nil {
		return "", err
	}
	return idAdjectives[a] + "-" + idNouns[n], nil
}

func pick(n int) (int, error) {
	i, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		return 0, fmt.Errorf("plan id: %w", err)
	}
	return int(i.Int64()), nil
}

// taken reports whether id names a plan in the store, stored or consumed: a consumed plan's id is
// never reused, even after its plan file is gone.
func (s Store) taken(id string) (bool, error) {
	for _, path := range []string{s.Path(id), s.MarkerPath(id)} {
		_, err := os.Lstat(path)
		if err == nil {
			return true, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return false, err
		}
	}
	return false, nil
}

// SaveNew stores a new plan under an id no plan in the store has used. It draws an id, skips it
// if a stored plan or a consumed marker already has it, builds the plan with build and saves it by
// exclusive creation; a save that loses a race for the id draws again. It gives up after maxDraws
// draws.
func (s Store) SaveNew(build func(id string) (Plan, error)) (Plan, error) {
	for range maxDraws {
		id, err := drawID()
		if err != nil {
			return Plan{}, err
		}
		if used, err := s.taken(id); err != nil {
			return Plan{}, err
		} else if used {
			continue
		}
		p, err := build(id)
		if err != nil {
			return Plan{}, err
		}
		if _, err := s.Save(p); errors.Is(err, fs.ErrExist) {
			continue
		} else if err != nil {
			return Plan{}, err
		}
		return p, nil
	}
	return Plan{}, fmt.Errorf("no free plan id after %d draws: %s holds too many plans; expired plans and their consumed markers can be removed", maxDraws, s.Dir)
}
