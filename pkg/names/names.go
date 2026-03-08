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
	// Characters — Commonwealth Saga (Peter F. Hamilton)
	"ozzie", "sheldon", "myo", "kime", "burnelli",
	"kazimir", "mellanie", "morton", "bose", "edeard",
	"slvasta", "qatux", "tochee", "johansson", "halgarth",
	"justine", "gore", "nigel", "paula", "stig",
	// Characters — Dune (Frank Herbert)
	"leto", "chani", "jessica", "stilgar", "thufir",
	"gurney", "feyd", "rabban", "yueh", "irulan",
	"shaddam", "odrade", "taraza", "scytale", "teg",
	"hayt", "kynes", "jamis", "bijaz", "korba",
	// Characters — Hyperion Cantos (Dan Simmons)
	"kassad", "lamia", "silenus", "hoyt", "weintraub",
	"gladstone", "aenea", "nemes", "ummon", "albedo",
	"brawne", "lenar", "fedmahn", "meina", "arundez",
	"dure", "moneta", "siri",
	// Characters — Foundation + Robots (Asimov)
	"salvor", "bayta", "toran", "ebling", "arcadia",
	"magnifico", "channis", "demerzel", "baley", "fastolfe",
	"gladia", "vasilia", "amadiro", "trevize", "pelorat",
	"fallom", "compor", "branno", "palver", "novi", "jander",
	// Characters — Hitchhiker's Guide (Douglas Adams)
	"zaphod", "trillian", "marvin", "fenchurch", "zarniwoop",
	"agrajag", "wowbagger", "hotblack", "prosser", "hactar",
	"jeltz", "desiato", "prefect", "tricia", "prak",
	// Characters — Culture (Iain M. Banks)
	"gurgeh", "zakalwe", "horza", "quilan", "diziet",
	"djan", "linter", "kraiklyn", "balveda", "aviger",
	"oramen", "ferbin", "hippinse", "vateuil", "ziller", "genar",
	// Characters — Discworld (Terry Pratchett)
	"vetinari", "vimes", "angua", "ridcully", "tiffany",
	"rincewind", "weatherwax", "ogg", "nobby", "detritus",
	"dorfl", "dibbler", "stibbons", "conina", "teatime",
	"lipwig", "nitt",
	// Characters — The Expanse (James S.A. Corey)
	"holden", "naomi", "amos", "bobbie", "avasarala",
	"drummer", "ashford", "dawes", "prax", "clarissa",
	"peaches", "klaes", "camina", "filip", "marco", "duarte",
	// Characters — misc
	"hari", "daneel", "gaal", "muaddib", "alia",
	"deckard", "case", "genly", "shevek", "wintermute",
	"montag", "nemo", "elijah", "hal", "hiro",
	"molly", "breq", "essun", "ender", "valentine",
	"ripley", "solaris", "pris", "neuromancer", "dawn",
	"lilith", "seivarden",
}
