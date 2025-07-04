// Code generated by gowebx, DO AVOID EDIT.
package tokencache

import (
	"github.com/ixugo/goddd/domain/token"
	"github.com/ixugo/goddd/pkg/conc"
)

var _ token.Storer = (*Cache)(nil)

func NewCache(store token.Storer, cache conc.Cacher) *Cache {
	return &Cache{
		store: store,
		token: cache,
	}
}

type Cache struct {
	store token.Storer
	token conc.Cacher
}

// Token implements token.TokenStorer
func (c *Cache) Token() token.TokenStorer {
	return (*Token)(c)
}
