// backend/internal/tests/integration/kb_test.go
// platform/kb against real Postgres. The two claims that need a live schema:
// that an unpublished article is excluded in SQL rather than by the caller,
// and that full-text search actually matches through the GIN expression
// index rather than degrading to a scan that finds nothing.
// Gate: INTEGRATION=1
package integration

import (
	"context"
	"testing"

	"github.com/mridha/businesssaas/internal/platform/kb"
)

type kbFixture struct {
	orgID      string
	ownerID    string
	memberID   string
	categoryID string
}

func seedKBFixture(t *testing.T, env *testEnv) *kbFixture {
	t.Helper()
	ctx := context.Background()
	orgID, _, ownerID := seedScopeTestOrg(t, env)
	memberID := seedOrgMember(t, env, orgID, "member", "kb-member")

	cat, err := env.kbSvc.CreateCategory(ctx, orgID, ownerID, kb.CreateCategoryRequest{
		Name: "IT Policies " + uniqueSlug("kbcat"),
	})
	if err != nil {
		t.Fatalf("create kb category: %v", err)
	}
	return &kbFixture{orgID: orgID, ownerID: ownerID, memberID: memberID, categoryID: cat.ID}
}

func seedArticle(t *testing.T, env *testEnv, fx *kbFixture, title, body string, publish bool) *kb.Article {
	t.Helper()
	ctx := context.Background()
	a, err := env.kbSvc.CreateArticle(ctx, fx.orgID, fx.ownerID, kb.CreateArticleRequest{
		CategoryID: &fx.categoryID, Title: title, Body: body,
	})
	if err != nil {
		t.Fatalf("create article %q: %v", title, err)
	}
	if publish {
		a, err = env.kbSvc.Publish(ctx, fx.orgID, fx.ownerID, a.ID)
		if err != nil {
			t.Fatalf("publish article %q: %v", title, err)
		}
	}
	return a
}

// ============================================================
// The structural claim — drafts are excluded in SQL
// ============================================================

// TestIntegration_KB_DraftsInvisibleWithoutManage is the confidentiality
// claim. A half-written HR policy read as authoritative is worse than no
// article, and the exclusion is a WHERE clause the caller cannot skip.
func TestIntegration_KB_DraftsInvisibleWithoutManage(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedKBFixture(t, env)

	published := seedArticle(t, env, fx, "Expense claim policy", "Submit within 30 days.", true)
	draft := seedArticle(t, env, fx, "Draft relocation policy", "Not finished yet.", false)

	res, err := env.kbSvc.ListArticles(ctx, fx.orgID, fx.memberID, kb.ArticleFilter{})
	if err != nil {
		t.Fatalf("list as member: %v", err)
	}
	for _, a := range res.Articles {
		if a.ID == draft.ID {
			t.Error("a member's list included an unpublished draft")
		}
		if a.Status != kb.StatusPublished {
			t.Errorf("member saw a %s article", a.Status)
		}
	}
	if res.Total != len(res.Articles) {
		t.Errorf("Total %d disagrees with the %d rows returned — count and list predicates have drifted",
			res.Total, len(res.Articles))
	}

	// The single-row read must agree with the list.
	if _, err := env.kbSvc.GetArticle(ctx, fx.orgID, fx.memberID, draft.ID); err == nil {
		t.Error("a member fetched a draft by id despite it being hidden from their list")
	}
	if _, err := env.kbSvc.GetArticle(ctx, fx.orgID, fx.memberID, published.ID); err != nil {
		t.Errorf("a member could not read a published article: %v", err)
	}

	// A manage holder sees both.
	all, err := env.kbSvc.ListArticles(ctx, fx.orgID, fx.ownerID, kb.ArticleFilter{})
	if err != nil {
		t.Fatalf("list as owner: %v", err)
	}
	seen := map[string]bool{}
	for _, a := range all.Articles {
		seen[a.ID] = true
	}
	if !seen[published.ID] || !seen[draft.ID] {
		t.Error("a manage holder did not see both the published article and the draft")
	}
}

// TestIntegration_KB_RepositoryDefaultExcludesDrafts goes below the service.
// The safe state is the DEFAULT: a zero-valued filter must return published
// articles only, so the failure mode of forgetting to configure it is
// "too little", never "too much".
func TestIntegration_KB_RepositoryDefaultExcludesDrafts(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedKBFixture(t, env)
	draft := seedArticle(t, env, fx, "Unfinished", "Draft body.", false)

	repo := kb.NewRepository(env.db)
	list, err := repo.FindArticles(ctx, fx.orgID, kb.ArticleFilter{})
	if err != nil {
		t.Fatalf("FindArticles with a zero filter: %v", err)
	}
	for _, a := range list {
		if a.ID == draft.ID {
			t.Fatal("a zero-valued ArticleFilter returned a draft — the default must be the safe state")
		}
	}
	got, err := repo.FindArticleByRef(ctx, fx.orgID, draft.ID, false)
	if err != nil {
		t.Fatalf("FindArticleByRef: %v", err)
	}
	if got != nil {
		t.Error("FindArticleByRef returned a draft with includeUnpublished=false")
	}
}

// ============================================================
// Search
// ============================================================

// TestIntegration_KB_FullTextSearch proves search works through the GIN
// expression index in migration 00112 — matching stemmed words in the body,
// not just literal titles. A knowledge base nobody can search is a filing
// cabinet with no labels.
func TestIntegration_KB_FullTextSearch(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedKBFixture(t, env)

	vpn := seedArticle(t, env, fx, "Remote access setup",
		"Install the VPN client and authenticate with your directory credentials.", true)
	seedArticle(t, env, fx, "Expense claim policy",
		"Submit receipts within thirty days of the expenditure.", true)

	// Matches the BODY, not the title — the whole point of indexing both.
	res, err := env.kbSvc.ListArticles(ctx, fx.orgID, fx.memberID, kb.ArticleFilter{Query: "VPN"})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(res.Articles) != 1 || res.Articles[0].ID != vpn.ID {
		t.Fatalf("search for \"VPN\" returned %d articles, want just the remote-access one", len(res.Articles))
	}

	// Stemming: "authenticated" must find "authenticate".
	res, err = env.kbSvc.ListArticles(ctx, fx.orgID, fx.memberID, kb.ArticleFilter{Query: "authenticated"})
	if err != nil {
		t.Fatalf("stemmed search: %v", err)
	}
	if len(res.Articles) != 1 {
		t.Errorf("stemmed search returned %d articles, want 1 — to_tsvector is not being applied", len(res.Articles))
	}

	// Punctuation must be treated as text, not tsquery syntax.
	if _, err := env.kbSvc.ListArticles(ctx, fx.orgID, fx.memberID, kb.ArticleFilter{Query: "VPN & !client"}); err != nil {
		t.Errorf("a query containing tsquery operators errored: %v — plainto_tsquery should treat them as words", err)
	}

	// Search must not become a way to reach drafts.
	seedArticle(t, env, fx, "Secret VPN rollout", "VPN replacement plan, not announced.", false)
	res, err = env.kbSvc.ListArticles(ctx, fx.orgID, fx.memberID, kb.ArticleFilter{Query: "VPN"})
	if err != nil {
		t.Fatalf("search after draft: %v", err)
	}
	if len(res.Articles) != 1 {
		t.Errorf("search returned %d articles for a member, want 1 — a draft matched the query", len(res.Articles))
	}
}

// ============================================================
// Lifecycle
// ============================================================

// TestIntegration_KB_PublishArchiveRepublish proves published_at records
// when the article FIRST went live, not when it was last touched — otherwise
// restoring an archived article jumps it to the top of the list as if newly
// written.
func TestIntegration_KB_PublishArchiveRepublish(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedKBFixture(t, env)
	a := seedArticle(t, env, fx, "Laptop refresh cycle", "Every three years.", true)

	firstPublished := a.PublishedAt
	if firstPublished == nil {
		t.Fatal("publish did not stamp published_at")
	}
	if _, err := env.kbSvc.Publish(ctx, fx.orgID, fx.ownerID, a.ID); err == nil {
		t.Error("publishing an already-published article was accepted")
	}

	archived, err := env.kbSvc.Archive(ctx, fx.orgID, fx.ownerID, a.ID)
	if err != nil {
		t.Fatalf("archive: %v", err)
	}
	if archived.Status != kb.StatusArchived {
		t.Errorf("status = %s, want archived", archived.Status)
	}
	// Archived is not deleted — superseded guidance still explains why
	// somebody acted the way they did.
	if _, err := env.kbSvc.GetArticle(ctx, fx.orgID, fx.ownerID, a.ID); err != nil {
		t.Errorf("an archived article is unreadable even to a manage holder: %v", err)
	}
	if _, err := env.kbSvc.GetArticle(ctx, fx.orgID, fx.memberID, a.ID); err == nil {
		t.Error("a member can still read an archived article")
	}

	restored, err := env.kbSvc.Publish(ctx, fx.orgID, fx.ownerID, a.ID)
	if err != nil {
		t.Fatalf("republish: %v", err)
	}
	if restored.PublishedAt == nil || !restored.PublishedAt.Equal(*firstPublished) {
		t.Errorf("published_at moved from %v to %v on republish — it records first publication, not last edit",
			firstPublished, restored.PublishedAt)
	}
}

// TestIntegration_KB_EditingAPublishedArticleKeepsItPublished — correcting a
// live policy must not silently unpublish it and leave employees reading
// nothing.
func TestIntegration_KB_EditingAPublishedArticleKeepsItPublished(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedKBFixture(t, env)
	a := seedArticle(t, env, fx, "Parking policy", "Spaces are first come first served.", true)

	body := "Spaces are allocated by ballot each quarter."
	updated, err := env.kbSvc.UpdateArticle(ctx, fx.orgID, fx.ownerID, a.ID, kb.UpdateArticleRequest{Body: &body})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Status != kb.StatusPublished {
		t.Errorf("status = %s after an edit, want it to stay published", updated.Status)
	}
	if _, err := env.kbSvc.GetArticle(ctx, fx.orgID, fx.memberID, a.ID); err != nil {
		t.Errorf("a member lost access to a live policy because it was corrected: %v", err)
	}

	// The correction must be searchable immediately — the expression index
	// is recomputed on write, which is the one place a derived value cannot
	// go stale.
	res, err := env.kbSvc.ListArticles(ctx, fx.orgID, fx.memberID, kb.ArticleFilter{Query: "ballot"})
	if err != nil {
		t.Fatalf("search after edit: %v", err)
	}
	if len(res.Articles) != 1 {
		t.Errorf("the edited body is not searchable: got %d results for \"ballot\"", len(res.Articles))
	}
}

// TestIntegration_KB_MemberCannotWrite proves .view alone is read-only.
func TestIntegration_KB_MemberCannotWrite(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedKBFixture(t, env)

	if _, err := env.kbSvc.CreateArticle(ctx, fx.orgID, fx.memberID, kb.CreateArticleRequest{
		Title: "Unauthorised", Body: "Body",
	}); err == nil {
		t.Error("a member created an article")
	}
	if _, err := env.kbSvc.CreateCategory(ctx, fx.orgID, fx.memberID, kb.CreateCategoryRequest{
		Name: "Unauthorised",
	}); err == nil {
		t.Error("a member created a category")
	}
	a := seedArticle(t, env, fx, "Read only", "Body.", true)
	if _, err := env.kbSvc.Publish(ctx, fx.orgID, fx.memberID, a.ID); err == nil {
		t.Error("a member published an article")
	}
}

// TestIntegration_KB_ArticlesAreBornDrafts — the first save of an article is
// the least likely to be the one worth publishing.
func TestIntegration_KB_ArticlesAreBornDrafts(t *testing.T) {
	env := newTestEnv(t)
	fx := seedKBFixture(t, env)
	a, err := env.kbSvc.CreateArticle(context.Background(), fx.orgID, fx.ownerID, kb.CreateArticleRequest{
		Title: "Newly written", Body: "Body.",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if a.Status != kb.StatusDraft {
		t.Errorf("status = %s on creation, want draft", a.Status)
	}
	if a.PublishedAt != nil {
		t.Error("a brand-new article has a published_at")
	}
}

// TestIntegration_KB_NoViewCountColumn — migration 00112 deliberately omits
// it. A counter nothing can recompute is unauditable the moment it drifts.
func TestIntegration_KB_NoViewCountColumn(t *testing.T) {
	env := newTestEnv(t)
	for _, col := range []string{"view_count", "views", "read_count", "helpful_count", "search_vector"} {
		var n int
		if err := env.db.QueryRow(context.Background(),
			`SELECT COUNT(*) FROM information_schema.columns
			  WHERE table_name = 'platform_kb_articles' AND column_name = $1`, col).Scan(&n); err != nil {
			t.Fatalf("introspect %s: %v", col, err)
		}
		if n != 0 {
			t.Errorf("platform_kb_articles.%s exists — see migration 00112 on why it should not", col)
		}
	}
}

// TestIntegration_KB_TenantIsolation
func TestIntegration_KB_TenantIsolation(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	a := seedKBFixture(t, env)
	b := seedKBFixture(t, env)
	article := seedArticle(t, env, a, "Org A only", "Confidential to org A.", true)

	if _, err := env.kbSvc.GetArticle(ctx, b.orgID, b.ownerID, article.ID); err == nil {
		t.Error("org B read org A's article")
	}
	res, err := env.kbSvc.ListArticles(ctx, b.orgID, b.ownerID, kb.ArticleFilter{Query: "Confidential"})
	if err != nil {
		t.Fatalf("search as org B: %v", err)
	}
	if len(res.Articles) != 0 {
		t.Error("org B's search matched org A's article")
	}
}
