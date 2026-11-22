package main

import (
	"encoding/json"
	"html/template"
)

// page is one page of the site. Its body is content/pages/<File>.md,
// served as HTML at Path and as Markdown at Path + ".md" (the home page's
// Markdown is /index.md).
type page struct {
	Path        string
	File        string
	Title       string // the <title>, 30 to 60 characters
	Description string // the meta description
	Crumb       string // the page's name in the breadcrumb and the nav
	Kind        string // home, article, faq, howto or page
	NoIndex     bool   // kept out of search engines and the sitemap
	Updated     string // the date the page last changed, YYYY-MM-DD
	// A blog post's date, and the page above it in the breadcrumb.
	Published              string
	ParentPath, ParentName string
}

// pages lists the site, in the order of its navigation.
var pages = []page{
	{
		Path: "/", File: "home", Kind: "home", Crumb: "Overview", Updated: "2026-11-22",
		Title:       "GoPatina: compile Go to Rust, no garbage collector",
		Description: "GoPatina is an open-source Go toolchain front end: write ordinary Go and, in an opt-in mode, compile it through Rust with no garbage collector.",
	},
	{
		Path: "/how-it-works", File: "how-it-works", Kind: "article", Crumb: "How it works", Updated: "2026-11-22",
		Title:       "How GoPatina compiles Go to Rust, with a borrow checker",
		Description: "How GoPatina turns Go into Rust: Go on top, a borrow checker that infers ownership in Go terms, rustc and LLVM underneath, and no garbage collector.",
	},
	{
		Path: "/get-started", File: "get-started", Kind: "howto", Crumb: "Get started", Updated: lastUpdated,
		Title:       "Get started with GoPatina: build Go through Rust",
		Description: "Install GoPatina and build a Go program three ways: as plain Go, as a readable Rust crate, and as a native binary built by rustc with no GC.",
	},
	{
		Path: "/evidence", File: "evidence", Kind: "article", Crumb: "Evidence", Updated: "2026-11-22",
		Title:       "GoPatina evidence: speed and correctness against Rust",
		Description: "GoPatina's evidence as a scientific report: hypotheses, method, apparatus, TOST equivalence at 5σ, a Six Sigma defect bound at 6σ, every result.",
	},
	{
		Path: "/benchmarks", File: "benchmarks", Kind: "article", Crumb: "Benchmarks", Updated: "2026-11-22",
		Title:       "GoPatina benchmarks: Go vs Rust performance, measured",
		Description: "How fast is Go compiled through Rust? GoPatina's benchmarks against hand-written Rust and plain Go: the method, measured ratios and caveats.",
	},
	{
		Path: "/correctness", File: "correctness", Kind: "article", Crumb: "Correctness", Updated: "2026-11-22",
		Title:       "GoPatina correctness: differential tests against Go",
		Description: "How GoPatina checks that Go compiled through Rust behaves exactly like Go: golden programs, generated differential tests and a Six Sigma bar.",
	},
	{
		Path: "/roadmap", File: "roadmap", Kind: "article", Crumb: "Roadmap", Updated: lastUpdated,
		Title:       "GoPatina roadmap: from a Go subset to goroutines",
		Description: "GoPatina's roadmap, phase by phase: the phase 1 subset of Go compiled through Rust, the rest of the borrow checker, goroutines, the standard library.",
	},
	{
		Path: "/faq", File: "faq", Kind: "faq", Crumb: "FAQ", Updated: "2026-11-22",
		Title:       "GoPatina FAQ: Go without a garbage collector, answered",
		Description: "Answers about GoPatina: what it is, whether it works today, whether your Go code still works, whether you need Rust, speed, correctness, license.",
	},
	{
		Path: "/sponsor", File: "sponsor", Kind: "page", Crumb: "Sponsor", Updated: "2026-11-22",
		Title:       "Sponsor GoPatina: support Go through Rust, no GC",
		Description: "Sponsor GoPatina, the open-source Go toolchain front end that compiles Go through Rust: tiers for individuals and companies, and what they pay for.",
	},
}

// notFoundPage is served, with status 404, for every unknown address.
var notFoundPage = page{
	Path: "/404", File: "404", Kind: "page", Crumb: "Page not found", NoIndex: true, Updated: lastUpdated,
	Title:       "Page not found: GoPatina, Go compiled through Rust",
	Description: "This address has no page. GoPatina's site now has several pages: overview, how it works, get started, evidence, benchmarks, correctness and FAQ.",
}

// references are the works the evidence report cites.
var references = []map[string]any{
	{"@type": "ScholarlyArticle", "name": "A comparison of the two one-sided tests procedure and the power approach for assessing the equivalence of average bioavailability", "author": "D. J. Schuirmann", "datePublished": "1987", "isPartOf": "Journal of Pharmacokinetics and Biopharmaceutics 15(6), 657–680"},
	{"@type": "ScholarlyArticle", "name": "If nothing goes wrong, is everything all right? Interpreting zero numerators", "author": "J. A. Hanley and A. Lippman-Hand", "datePublished": "1983", "isPartOf": "JAMA 249(13), 1743–1745"},
	{"@type": "Book", "name": "The Nature of Six Sigma Quality", "author": "M. J. Harry", "datePublished": "1988", "publisher": "Motorola University Press"},
}

const (
	orgID  = "https://pigfox.com/#organization"
	siteID = siteURL + "#website"
	softID = siteURL + "#software"
)

// url returns the page's canonical URL.
func (p page) url() string { return siteURL + p.Path[1:] }

// markdownPath returns where the page's Markdown is served.
func (p page) markdownPath() string {
	if p.Path == "/" {
		return "/index.md"
	}
	return p.Path + ".md"
}

// jsonLD returns the page's structured data: the publisher, the site, the
// page, its breadcrumb (for inner pages) and what the page is.
func (p page) jsonLD(d doc) template.JS {
	u := p.url()
	webpage := map[string]any{
		"@type": "WebPage", "@id": u + "#webpage", "url": u, "name": p.Title,
		"description": p.Description, "isPartOf": map[string]any{"@id": siteID},
		"dateModified": p.Updated, "inLanguage": "en",
	}
	graph := []any{
		map[string]any{"@type": "Organization", "@id": orgID, "name": "Pigfox LLC", "url": "https://pigfox.com"},
		map[string]any{"@type": "WebSite", "@id": siteID, "name": "GoPatina", "url": siteURL, "inLanguage": "en", "publisher": map[string]any{"@id": orgID}},
		webpage,
	}
	author := map[string]any{"@type": "Person", "name": "Peter Sjolin", "url": "https://github.com/pigfox"}
	if p.Path != "/" {
		webpage["breadcrumb"] = map[string]any{"@id": u + "#breadcrumb"}
		items := []any{map[string]any{"@type": "ListItem", "position": 1, "name": "GoPatina", "item": siteURL}}
		if p.ParentPath != "" {
			items = append(items, map[string]any{"@type": "ListItem", "position": 2, "name": p.ParentName, "item": siteURL + p.ParentPath[1:]})
		}
		items = append(items, map[string]any{"@type": "ListItem", "position": len(items) + 1, "name": p.Crumb, "item": u})
		graph = append(graph, map[string]any{"@type": "BreadcrumbList", "@id": u + "#breadcrumb", "itemListElement": items})
	}
	switch p.Kind {
	case "home":
		webpage["about"] = map[string]any{"@id": softID}
		graph = append(graph, map[string]any{
			"@type": []string{"SoftwareSourceCode", "SoftwareApplication"}, "@id": softID,
			"name": "GoPatina", "url": siteURL,
			"description":         "A Go toolchain front end. Without a flag it runs the installed Go toolchain unchanged; opt-in modes compile a defined subset of Go through Rust, with no garbage collector. Phase 1 of the roadmap is implemented for that subset.",
			"codeRepository":      "https://github.com/pigfox/gopatina",
			"programmingLanguage": "Go",
			"license":             "https://github.com/pigfox/gopatina#license",
			"applicationCategory": "DeveloperApplication",
			"operatingSystem":     "Linux, macOS",
			"softwareVersion":     "0.0.1",
			"author":              author,
			"publisher":           map[string]any{"@id": orgID},
		})
	case "article":
		article := map[string]any{
			"@type": "TechArticle", "@id": u + "#article", "headline": d.Title,
			"description": p.Description, "author": author, "publisher": map[string]any{"@id": orgID},
			"dateModified": p.Updated, "mainEntityOfPage": map[string]any{"@id": u + "#webpage"},
			"about": map[string]any{"@id": softID}, "inLanguage": "en",
		}
		if p.Path == "/evidence" {
			article["citation"] = references
		}
		graph = append(graph, article)
	case "faq":
		webpage["@type"] = []string{"WebPage", "FAQPage"}
		var qs []any
		for _, s := range d.Sections {
			qs = append(qs, map[string]any{
				"@type": "Question", "name": s.Heading,
				"acceptedAnswer": map[string]any{"@type": "Answer", "text": s.Text},
			})
		}
		webpage["mainEntity"] = qs
	case "blog":
		var posts []any
		for _, q := range d.Posts {
			posts = append(posts, map[string]any{
				"@type": "BlogPosting", "@id": q.url() + "#post", "headline": q.Title, "url": q.url(),
				"datePublished": q.Published, "dateModified": q.Updated,
			})
		}
		graph = append(graph, map[string]any{
			"@type": "Blog", "@id": u + "#blog", "name": "GoPatina blog", "url": u,
			"description": p.Description, "author": author, "publisher": map[string]any{"@id": orgID},
			"mainEntityOfPage": map[string]any{"@id": u + "#webpage"}, "inLanguage": "en", "blogPost": posts,
		})
	case "post":
		graph = append(graph, map[string]any{
			"@type": "BlogPosting", "@id": u + "#post", "headline": d.Title, "description": p.Description,
			"datePublished": p.Published, "dateModified": p.Updated, "author": author,
			"publisher": map[string]any{"@id": orgID}, "mainEntityOfPage": map[string]any{"@id": u + "#webpage"},
			"isPartOf": map[string]any{"@id": blogPage.url() + "#blog"}, "about": map[string]any{"@id": softID},
			"inLanguage": "en",
		})
	case "howto":
		var steps []any
		for i, s := range d.Steps {
			steps = append(steps, map[string]any{"@type": "HowToStep", "position": i + 1, "text": s})
		}
		graph = append(graph, map[string]any{
			"@type": "HowTo", "@id": u + "#howto", "name": d.Title, "description": p.Description,
			"step": steps, "mainEntityOfPage": map[string]any{"@id": u + "#webpage"},
		})
	}
	out, _ := json.MarshalIndent(map[string]any{"@context": "https://schema.org", "@graph": graph}, "", "  ")
	return template.JS(out)
}
