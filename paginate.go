package einvoice

import "context"

// Paginate iterates every item of a paged list, fetching page after page. list is called with the
// 1-based page number and returns that page (set it on the query you close over); fn is called for
// each item and stops the walk by returning an error, which Paginate returns as is. The walk ends
// when a page says it has no next page, or comes back empty.
//
//	query := &einvoice.ListBuyersQuery{Limit: einvoice.Ptr(100.0)}
//	err := einvoice.Paginate(ctx, func(ctx context.Context, page int64) (*einvoice.Page[einvoice.BuyerViewDto], error) {
//		query.Page = einvoice.Ptr(float64(page))
//		return client.Buyers.List(ctx, query)
//	}, func(buyer einvoice.BuyerViewDto) error {
//		fmt.Println(buyer.Name)
//		return nil
//	})
func Paginate[T any](ctx context.Context, list func(ctx context.Context, page int64) (*Page[T], error), fn func(item T) error) error {
	for page := int64(1); ; page++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		result, err := list(ctx, page)
		if err != nil {
			return err
		}
		if result == nil {
			return nil
		}
		for _, item := range result.Data {
			if err := fn(item); err != nil {
				return err
			}
		}
		if !result.Pagination.HasNext || len(result.Data) == 0 {
			return nil
		}
	}
}

// Collect gathers every item of a paged list into one slice (see Paginate).
func Collect[T any](ctx context.Context, list func(ctx context.Context, page int64) (*Page[T], error)) ([]T, error) {
	var out []T
	err := Paginate(ctx, list, func(item T) error {
		out = append(out, item)
		return nil
	})
	return out, err
}
