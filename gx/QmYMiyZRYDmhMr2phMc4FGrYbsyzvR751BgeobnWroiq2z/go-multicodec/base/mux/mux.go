package basemux

import (
	mc "mbfs/go-mbfs/gx/QmYMiyZRYDmhMr2phMc4FGrYbsyzvR751BgeobnWroiq2z/go-multicodec"
	mux "mbfs/go-mbfs/gx/QmYMiyZRYDmhMr2phMc4FGrYbsyzvR751BgeobnWroiq2z/go-multicodec/mux"

	b64 "mbfs/go-mbfs/gx/QmYMiyZRYDmhMr2phMc4FGrYbsyzvR751BgeobnWroiq2z/go-multicodec/base/b64"
	bin "mbfs/go-mbfs/gx/QmYMiyZRYDmhMr2phMc4FGrYbsyzvR751BgeobnWroiq2z/go-multicodec/base/bin"
	hex "mbfs/go-mbfs/gx/QmYMiyZRYDmhMr2phMc4FGrYbsyzvR751BgeobnWroiq2z/go-multicodec/base/hex"
)

func AllBasesMux() *mux.Multicodec {
	m := mux.MuxMulticodec([]mc.Multicodec{
		hex.Multicodec(),
		b64.Multicodec(),
		bin.Multicodec(),
	}, mux.SelectFirst)
	m.Wrap = false
	return m
}
