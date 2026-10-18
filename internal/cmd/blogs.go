package cmd

import (
	"fmt"
	"strings"

	fopost "github.com/fopost/fopost-go"
	"github.com/spf13/cobra"

	"github.com/fopost/fopost-cli/internal/output"
)

func init() { register(newBlogsCmd) }

// blogs reaches content the connected site already owns — a WordPress site's
// posts or a Shopify store's blog articles and products — by the platform's own
// ids rather than FoPost ids.
func newBlogsCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "blogs",
		Aliases: []string{"blog"},
		Short:   "Read and manage articles and products on a connected site",
	}
	cmd.AddCommand(
		newBlogsListCmd(state),
		newArticlesCmd(state),
		newProductsCmd(state),
	)
	return cmd
}

func newBlogsListCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "list <account-id>",
		Short: "List the blogs on a connected site",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			blogs, err := client.Blogs.ListBlogs(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(blogs, func() {
				rows := make([][]string, 0, len(blogs))
				for _, blog := range blogs {
					rows = append(rows, []string{
						blog.ID,
						output.Truncate(blog.Title, 32),
						output.Dash(deref(blog.Handle)),
						output.Dash(deref(blog.URL)),
					})
				}
				printer.Table([]string{"id", "title", "handle", "url"}, rows)
			})
		},
	}
}

func newArticlesCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "articles",
		Aliases: []string{"article"},
		Short:   "List, read, write and remove articles on a blog",
	}
	cmd.AddCommand(
		newArticlesListCmd(state),
		newArticlesGetCmd(state),
		newArticlesCreateCmd(state),
		newArticlesUpdateCmd(state),
		newArticlesDeleteCmd(state),
	)
	return cmd
}

func newArticlesListCmd(state *State) *cobra.Command {
	var limit int
	var status, query string
	cmd := &cobra.Command{
		Use:   "list <account-id> <blog-id>",
		Short: "List a blog's articles, newest first, drafts included",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			articles, err := client.Blogs.ListArticles(cmd.Context(), args[0], args[1], &fopost.ListArticlesParams{
				Limit:  limit,
				Status: status,
				Q:      query,
			})
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(articles, func() {
				rows := make([][]string, 0, len(articles))
				for _, article := range articles {
					rows = append(rows, []string{
						article.ID,
						output.Truncate(article.Title, 40),
						article.Status,
						output.Dash(deref(article.PublishedAt)),
					})
				}
				printer.Table([]string{"id", "title", "status", "published"}, rows)
			})
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "how many to return, 1 to 50")
	cmd.Flags().StringVar(&status, "status", "", "published, draft, pending or scheduled")
	cmd.Flags().StringVar(&query, "query", "", "match the article title")
	return cmd
}

func newArticlesGetCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "get <account-id> <blog-id> <article-id>",
		Short: "Read one article",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			article, err := client.Blogs.GetArticle(cmd.Context(), args[0], args[1], args[2])
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(article, func() {
				printer.Table([]string{"field", "value"}, [][]string{
					{"id", article.ID},
					{"title", article.Title},
					{"status", article.Status},
					{"tags", output.Dash(strings.Join(article.Tags, ", "))},
					{"url", output.Dash(deref(article.URL))},
					{"updated", output.Dash(deref(article.UpdatedAt))},
				})
			})
		},
	}
}

func newArticlesCreateCmd(state *State) *cobra.Command {
	var title, body, excerpt, status, author, imageURL string
	var tags []string
	cmd := &cobra.Command{
		Use:   "create <account-id> <blog-id>",
		Short: "Write a new article to the site",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if title == "" || body == "" {
				return usageErrorf("--title and --body are required")
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			article, err := client.Blogs.CreateArticle(cmd.Context(), args[0], args[1], &fopost.ArticleRequest{
				Title:      title,
				Body:       body,
				Excerpt:    excerpt,
				Status:     status,
				Tags:       tags,
				AuthorName: author,
				ImageURL:   imageURL,
			})
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(article, func() {
				printer.Success("Created article %s (%s).", article.Title, article.ID)
			})
		},
	}
	articleFlags(cmd, &title, &body, &excerpt, &status, &author, &imageURL, &tags)
	return cmd
}

func newArticlesUpdateCmd(state *State) *cobra.Command {
	var title, body, excerpt, status, author, imageURL string
	var tags []string
	cmd := &cobra.Command{
		Use:   "update <account-id> <blog-id> <article-id>",
		Short: "Change a live article in place",
		Long: "Changes the article that is already on the site. Only the flags you pass " +
			"change, and the article is addressed by its own id, so this never creates a " +
			"second post.",
		Args: cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			// An update with no flags would send an empty body, which the API
			// rejects; saying so here costs no round trip.
			if title == "" && body == "" && excerpt == "" && status == "" &&
				author == "" && imageURL == "" && tags == nil {
				return usageErrorf("pass at least one field to change")
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			article, err := client.Blogs.UpdateArticle(cmd.Context(), args[0], args[1], args[2], &fopost.ArticleRequest{
				Title:      title,
				Body:       body,
				Excerpt:    excerpt,
				Status:     status,
				Tags:       tags,
				AuthorName: author,
				ImageURL:   imageURL,
			})
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(article, func() {
				printer.Success("Updated article %s.", article.ID)
			})
		},
	}
	articleFlags(cmd, &title, &body, &excerpt, &status, &author, &imageURL, &tags)
	return cmd
}

func newArticlesDeleteCmd(state *State) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete <account-id> <blog-id> <article-id>",
		Short: "Remove an article from the site",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				if err := confirm(state, fmt.Sprintf("Delete article %s from the site? This cannot be undone.", args[2])); err != nil {
					return err
				}
			}
			client, err := state.Client()
			if err != nil {
				return err
			}
			if err := client.Blogs.DeleteArticle(cmd.Context(), args[0], args[1], args[2]); err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(deleted{Deleted: true, ID: args[2]}, func() {
				printer.Success("Deleted article %s.", args[2])
			})
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation prompt")
	return cmd
}

func newProductsCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "products",
		Aliases: []string{"product"},
		Short:   "List and update the store's products",
	}
	cmd.AddCommand(newProductsListCmd(state), newProductsUpdateCmd(state))
	return cmd
}

func newProductsListCmd(state *State) *cobra.Command {
	var limit int
	var status, query string
	cmd := &cobra.Command{
		Use:   "list <account-id>",
		Short: "List the store's products",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			products, err := client.Blogs.ListProducts(cmd.Context(), args[0], &fopost.ListProductsParams{
				Limit:  limit,
				Status: status,
				Q:      query,
			})
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(products, func() {
				rows := make([][]string, 0, len(products))
				for _, product := range products {
					price := output.Dash(deref(product.Price))
					if product.Currency != nil && product.Price != nil {
						price = *product.Price + " " + *product.Currency
					}
					rows = append(rows, []string{
						product.ID,
						output.Truncate(product.Title, 40),
						product.Status,
						price,
					})
				}
				printer.Table([]string{"id", "title", "status", "price"}, rows)
			})
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "how many to return, 1 to 50")
	cmd.Flags().StringVar(&status, "status", "", "active, draft or archived")
	cmd.Flags().StringVar(&query, "query", "", "match the product title")
	return cmd
}

func newProductsUpdateCmd(state *State) *cobra.Command {
	var title, description, status, productType, vendor string
	var tags []string
	cmd := &cobra.Command{
		Use:   "update <account-id> <product-id>",
		Short: "Change a product on the store",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			product, err := client.Blogs.UpdateProduct(cmd.Context(), args[0], args[1], &fopost.ProductRequest{
				Title:       title,
				Description: description,
				Status:      status,
				Tags:        tags,
				ProductType: productType,
				Vendor:      vendor,
			})
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(product, func() {
				printer.Success("Updated product %s.", product.ID)
			})
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "product title")
	cmd.Flags().StringVar(&description, "description", "", "product description")
	cmd.Flags().StringVar(&status, "status", "", "active, draft or archived")
	cmd.Flags().StringVar(&productType, "product-type", "", "product type")
	cmd.Flags().StringVar(&vendor, "vendor", "", "vendor")
	cmd.Flags().StringSliceVar(&tags, "tag", nil, "product tag; repeat for several")
	return cmd
}

func articleFlags(cmd *cobra.Command, title, body, excerpt, status, author, imageURL *string, tags *[]string) {
	cmd.Flags().StringVar(title, "title", "", "article title")
	cmd.Flags().StringVar(body, "body", "", "article body, in FoPost body markup")
	cmd.Flags().StringVar(excerpt, "excerpt", "", "summary shown in listings")
	cmd.Flags().StringVar(status, "status", "", "published, draft, pending or scheduled")
	cmd.Flags().StringVar(author, "author", "", "byline, where the platform accepts one")
	cmd.Flags().StringVar(imageURL, "image-url", "", "public http(s) URL of the featured image")
	cmd.Flags().StringSliceVar(tags, "tag", nil, "article tag; repeat for several")
}
