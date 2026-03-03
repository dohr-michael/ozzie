// Package names generates friendly names for Ozzie entities (tasks, sessions, schedules).
// Names are themed around science-fiction literature: adjectives from space/exploration
// combined with SF author surnames and iconic character names.
package names

// left contains adjectives themed around space, exploration, and science.
var left = []string{
	"cosmic", "stellar", "quantum", "nebular", "orbital",
	"galactic", "astral", "lunar", "solar", "plasma",
	"photon", "quasar", "pulsar", "nova", "hyper",
	"warp", "void", "dark", "bright", "frozen",
	"silent", "drifting", "distant", "ancient", "radiant",
	"chrome", "crimson", "golden", "silver", "azure",
	"swift", "bold", "fierce", "calm", "vivid",
	"epic", "prime", "deep", "keen", "vast",
	"rapid", "subtle", "lucid", "cryptic", "arcane",
	"primal", "spectral", "temporal", "parallel", "inverse",
}

// right contains SF author surnames and iconic character names.
var right = []string{
	// Authors
	"asimov", "clarke", "hamilton", "herbert", "leguin",
	"dick", "bradbury", "verne", "wells", "heinlein",
	"gibson", "butler", "lem", "banks", "simmons",
	"haldeman", "bester", "zelazny", "wolfe", "stephenson",
	"jemisin", "leckie", "orwell", "huxley", "atwood",
	"ballard", "bear", "niven", "cherryh", "ellison",
	"sturgeon", "tiptree", "vance", "aldiss", "pohl",
	"silverberg", "robinson", "farmer", "delaney", "liu",
	"tchaikovsky", "corey", "scalzi", "reynolds", "baxter",
	"moorcock", "brunner",
	// Characters
	"hari", "daneel", "gaal", "muaddib", "alia",
	"deckard", "case", "genly", "shevek", "wintermute",
	"montag", "nemo", "elijah", "hal", "hiro",
	"molly", "breq", "essun", "ender", "valentine",
	"ripley", "solaris", "pris", "neuromancer", "dawn",
	"lilith", "seivarden",
	// Commonwealth Saga (Peter F. Hamilton)
	"ozzie", "sheldon", "myo", "kime", "burnelli",
	"kazimir", "mellanie", "morton", "bose", "edeard",
	"slvasta", "qatux", "tochee", "johansson", "halgarth",
	"justine", "gore", "nigel", "paula", "stig",
}
