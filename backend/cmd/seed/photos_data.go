package main

// seedPhotos are the hand-picked demo photos per product slug (see photos.go):
// photographs checked by eye, CC0 or public domain, from Openverse.
var seedPhotos = map[string][]photoSource{
	"achchiq-qalampir": {
		{URL: "https://live.staticflickr.com/65535/51554641678_4ec446f5c1_b.jpg", License: "cc0", Page: "https://www.flickr.com/photos/89918055@N05/51554641678"},
		{URL: "https://live.staticflickr.com/65535/55184793138_66d2c721a9_b.jpg", License: "cc0", Page: "https://www.flickr.com/photos/7633518@N08/55184793138"},
		{URL: "https://cdn.stocksnap.io/img-thumbs/960w/8002II1N54.jpg", License: "cc0", Page: "https://stocksnap.io/photo/chili-pepper-8002II1N54"},
	},
	"anjir": {
		{URL: "https://cdn.stocksnap.io/img-thumbs/960w/YVDRQAGWOP.jpg", License: "cc0", Page: "https://stocksnap.io/photo/figs-fruits-YVDRQAGWOP"},
	},
	"anor": {
		{URL: "https://live.staticflickr.com/65535/51487001887_2d16cb9afe_b.jpg", License: "pdm", Page: "https://www.flickr.com/photos/190270151@N02/51487001887"},
		{URL: "https://live.staticflickr.com/65535/54089946235_fa0dcb9e20_b.jpg", License: "cc0", Page: "https://www.flickr.com/photos/7633518@N08/54089946235"},
		{URL: "https://live.staticflickr.com/65535/52003056180_6cdb9e5cdc_b.jpg", License: "cc0", Page: "https://www.flickr.com/photos/43296902@N00/52003056180"},
	},
	"apelsin": {
		{URL: "https://cdn.stocksnap.io/img-thumbs/960w/C0KXFAPR8M.jpg", License: "cc0", Page: "https://stocksnap.io/photo/oranges-fruits-C0KXFAPR8M"},
		{URL: "https://cdn.stocksnap.io/img-thumbs/960w/31WR2ZXHQO.jpg", License: "cc0", Page: "https://stocksnap.io/photo/nature-plants-31WR2ZXHQO"},
		{URL: "https://cdn.stocksnap.io/img-thumbs/960w/FJC39SV4ZC.jpg", License: "cc0", Page: "https://stocksnap.io/photo/oranges-fruits-FJC39SV4ZC"},
	},
	"armatura": {
		{URL: "https://live.staticflickr.com/1676/25404129454_2d6daf4b30_b.jpg", License: "pdm", Page: "https://www.flickr.com/photos/42009447@N05/25404129454"},
		{URL: "https://live.staticflickr.com/65535/50982930286_fff18ee1be_b.jpg", License: "cc0", Page: "https://www.flickr.com/photos/96541566@N06/50982930286"},
	},
	"arpa": {
		{URL: "https://live.staticflickr.com/65535/51380202253_7b29729505_b.jpg", License: "cc0", Page: "https://www.flickr.com/photos/89918055@N05/51380202253"},
	},
	"asal": {
		{URL: "https://cdn.stocksnap.io/img-thumbs/960w/97LJAKWL36.jpg", License: "cc0", Page: "https://stocksnap.io/photo/honey-jars-97LJAKWL36"},
		{URL: "https://images.rawpixel.com/editor_1024/czNmcy1wcml2YXRlL3Jhd3BpeGVsX2ltYWdlcy93ZWJzaXRlX2NvbnRlbnQvbHIvZnJqZWxseV9wZWFjaF9qZWxseV9jYW5uaW5nLWltYWdlLWt5YmRjMGt2LmpwZw.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/6035650/honey-jar-free-public-domain-cc0-photo"},
		{URL: "https://live.staticflickr.com/65535/51330638681_1e5642f21a_b.jpg", License: "cc0", Page: "https://www.flickr.com/photos/184594136@N08/51330638681"},
		{URL: "https://live.staticflickr.com/65535/51330848773_92478805dd_b.jpg", License: "cc0", Page: "https://www.flickr.com/photos/184594136@N08/51330848773"},
	},
	"baliq": {
		{URL: "https://live.staticflickr.com/65535/51731916285_0cc8b596ce_b.jpg", License: "pdm", Page: "https://www.flickr.com/photos/51986662@N05/51731916285"},
		{URL: "https://images.rawpixel.com/editor_1024/czNmcy1wcml2YXRlL3Jhd3BpeGVsX2ltYWdlcy93ZWJzaXRlX2NvbnRlbnQvbHIvaXM5Mjk1LWltYWdlLWt3dnlldXlkLmpwZw.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/5926652/photo-image-public-domain-food-water"},
		{URL: "https://images.rawpixel.com/editor_1024/czNmcy1wcml2YXRlL3Jhd3BpeGVsX2ltYWdlcy93ZWJzaXRlX2NvbnRlbnQvbHIvaXMxODEzOS1pbWFnZS1rd3Z3djA3dy5qcGc.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/5917444/image-public-domain-food-water"},
		{URL: "https://images.rawpixel.com/editor_1024/cHJpdmF0ZS9zdGF0aWMvaW1hZ2Uvd2Vic2l0ZS8yMDIyLTA0L2xyL3B4MTU4MzM0MS1pbWFnZS1rd3Z2b3I0ay5qcGc.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/5909922/image-light-public-domain-leaf"},
	},
	"banan": {
		{URL: "https://images.rawpixel.com/editor_1024/cHJpdmF0ZS9zdGF0aWMvaW1hZ2Uvd2Vic2l0ZS8yMDIyLTA0L2xyL2ZyYmFuYW5hc19mcnVpdF9iYW5hbmFfc2hydWItaW1hZ2Uta3liZWE2cDUuanBn.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/6041225/photo-image-public-domain-plant-fruit"},
		{URL: "https://images.rawpixel.com/editor_1024/czNmcy1wcml2YXRlL3Jhd3BpeGVsX2ltYWdlcy93ZWJzaXRlX2NvbnRlbnQvbHIvcHgxMDM5NDUxLWltYWdlLWt3dnk3dmYyLmpwZw.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/5925690/photo-image-public-domain-plant-green"},
		{URL: "https://images.rawpixel.com/editor_1024/cHJpdmF0ZS9zdGF0aWMvaW1hZ2Uvd2Vic2l0ZS8yMDIyLTA0L2xyL2ZyYmFuYW5hX2ZydWl0X2Zvb2RfdGFibGUtaW1hZ2Uta3liZWFnZTguanBn.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/6041248/photo-image-public-domain-plant-fruit"},
		{URL: "https://images.rawpixel.com/editor_1024/cHJpdmF0ZS9zdGF0aWMvaW1hZ2Uvd2Vic2l0ZS8yMDIyLTA0L2xyL2ZyYmFuYW5hc19mcnVpdHNfZnJ1aXRfaGVhbHRoeV85LWltYWdlLWt5YmM3b29qLmpwZw.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/6028955/photo-image-public-domain-plant-fruit"},
	},
	"bedana": {
		{URL: "https://images.rawpixel.com/editor_1024/czNmcy1wcml2YXRlL3Jhd3BpeGVsX2ltYWdlcy93ZWJzaXRlX2NvbnRlbnQvbHIvcHgxNDMxMTgxLWltYWdlLWt3dnh4cXNnLmpwZw.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/5923894/photo-image-public-domain-food-free"},
		{URL: "https://images.rawpixel.com/editor_1024/cHJpdmF0ZS9zdGF0aWMvaW1hZ2Uvd2Vic2l0ZS8yMDIyLTA0L2xyL2ZybWFya2V0X2VnZ19mb29kXzEyNjE0NDgtaW1hZ2Uta3liYjF4ZHEuanBn.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/6021139/quail-eggs-free-public-domain-cc0-photo"},
		{URL: "https://images.rawpixel.com/editor_1024/czNmcy1wcml2YXRlL3Jhd3BpeGVsX2ltYWdlcy93ZWJzaXRlX2NvbnRlbnQvc3YxOTY2NDQtaW1hZ2Uta3d2dWhieWouanBn.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/5902025/photo-image-background-public-domain-food"},
	},
	"behi": {
		{URL: "https://images.rawpixel.com/editor_1024/cHJpdmF0ZS9zdGF0aWMvaW1hZ2Uvd2Vic2l0ZS8yMDIyLTA0L2xyL2ZycXVpbmNlX2ZydWl0X3BsYW50X2xlYWZfMTMtaW1hZ2Uta3liY3doeW4uanBn.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/6033085/photo-image-public-domain-leaf-tree"},
		{URL: "https://live.staticflickr.com/65535/54783778093_4ae58d7437_b.jpg", License: "cc0", Page: "https://www.flickr.com/photos/96541566@N06/54783778093"},
		{URL: "https://images.rawpixel.com/editor_1024/cHJpdmF0ZS9zdGF0aWMvaW1hZ2Uvd2Vic2l0ZS8yMDIyLTA0L2xyL2ZycXVpbmNlX2ZydWl0X3BsYW50X2xlYWZfMi1pbWFnZS1reWJhdDNyNC5qcGc.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/6019491/photo-image-public-domain-leaf-tree"},
	},
	"bodom": {
		{URL: "https://images.rawpixel.com/editor_1024/cHJpdmF0ZS9zdGF0aWMvaW1hZ2Uvd2Vic2l0ZS8yMDIyLTA0L2xyL3B4MTQzNzk1My1pbWFnZS1rd3Z2dzU3Mi5qcGc.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/5910917/image-background-public-domain-white"},
		{URL: "https://images.rawpixel.com/editor_1024/czNmcy1wcml2YXRlL3Jhd3BpeGVsX2ltYWdlcy93ZWJzaXRlX2NvbnRlbnQvbHIvcHg3ODczNDYtaW1hZ2Uta3d2dXpjOWouanBn.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/5905344/photo-image-public-domain-plant-food"},
		{URL: "https://images.rawpixel.com/editor_1024/cHJpdmF0ZS9zdGF0aWMvaW1hZ2Uvd2Vic2l0ZS8yMDIyLTA0L2xyL3B4NTU2MTg5LWltYWdlLWt3dnZldGprLmpwZw.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/5908146/image-public-domain-plant-food"},
	},
	"bodring": {
		{URL: "https://live.staticflickr.com/65535/52232429724_100e345593_b.jpg", License: "cc0", Page: "https://www.flickr.com/photos/184594136@N08/52232429724"},
		{URL: "https://live.staticflickr.com/65535/52232164518_8a6abd39bc_b.jpg", License: "cc0", Page: "https://www.flickr.com/photos/184594136@N08/52232164518"},
		{URL: "https://live.staticflickr.com/65535/51859296155_055f3f67c2_b.jpg", License: "cc0", Page: "https://www.flickr.com/photos/184594136@N08/51859296155"},
	},
	"boyoq": {
		{URL: "https://pd.w.org/2022/03/587623dd9d6a718f3.62785904-2048x1281.jpg", License: "cc0", Page: "https://wordpress.org/photos/photo/587623dd9d/"},
	},
	"brokkoli": {
		{URL: "https://live.staticflickr.com/65535/52304010615_7ba22e5724_b.jpg", License: "cc0", Page: "https://www.flickr.com/photos/184594136@N08/52304010615"},
		{URL: "https://live.staticflickr.com/65535/55311042233_fa884e56fa_b.jpg", License: "pdm", Page: "https://www.flickr.com/photos/25802865@N08/55311042233"},
	},
	"bugdoy": {
		{URL: "https://images.rawpixel.com/editor_1024/cHJpdmF0ZS9zdGF0aWMvaW1hZ2Uvd2Vic2l0ZS8yMDIyLTA0L2xyL2ZyZWVyYW5nZXBleGVsczA2NTM0LWltYWdlLWt3dnd6cHBvLmpwZw.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/5918393/image-public-domain-food-free"},
		{URL: "https://images.rawpixel.com/editor_1024/cHJpdmF0ZS9zdGF0aWMvaW1hZ2Uvd2Vic2l0ZS8yMDIyLTA0L2xyL3B4MTE1NTA1Mi1pbWFnZS1rd3Z3OGVpci5qcGc.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/5913289/photo-image-public-domain-golden-free"},
		{URL: "https://cdn.stocksnap.io/img-thumbs/960w/ZTR6YUEDD7.jpg", License: "cc0", Page: "https://stocksnap.io/photo/wheat-grains-ZTR6YUEDD7"},
		{URL: "https://images.rawpixel.com/editor_1024/cHJpdmF0ZS9zdGF0aWMvaW1hZ2Uvd2Vic2l0ZS8yMDIyLTA0L2xyL2Zyd2hlYXRfd2hlYXRfZmllbGRfY2VyZWFscy1iXzE3LWltYWdlLWt5YmFhaWZ2LmpwZw.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/6016314/photo-image-public-domain-plant-green"},
	},
	"bulgor": {
		{URL: "https://live.staticflickr.com/65535/53964157901_b7ac8d4828_b.jpg", License: "cc0", Page: "https://www.flickr.com/photos/40632439@N00/53964157901"},
		{URL: "https://live.staticflickr.com/65535/55148676592_8f28fdfc6e_b.jpg", License: "cc0", Page: "https://www.flickr.com/photos/40632439@N00/55148676592"},
	},
	"choy": {
		{URL: "https://images.rawpixel.com/editor_1024/czNmcy1wcml2YXRlL3Jhd3BpeGVsX2ltYWdlcy93ZWJzaXRlX2NvbnRlbnQvcHgxMzI4MTMxLWltYWdlLWt3dncxYWxuLmpwZw.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/5911912/image-background-public-domain-leaf"},
		{URL: "https://live.staticflickr.com/65535/54763573482_820e11a71b_b.jpg", License: "pdm", Page: "https://www.flickr.com/photos/76025239@N02/54763573482"},
		{URL: "https://live.staticflickr.com/65535/54764660723_1f4bd675c7_b.jpg", License: "cc0", Page: "https://www.flickr.com/photos/76025239@N02/54764660723"},
	},
	"gilos": {
		{URL: "https://cdn.stocksnap.io/img-thumbs/960w/YS8L3DPSXI.jpg", License: "cc0", Page: "https://stocksnap.io/photo/red-cherries-YS8L3DPSXI"},
		{URL: "https://cdn.stocksnap.io/img-thumbs/960w/MYRKEUABZY.jpg", License: "cc0", Page: "https://stocksnap.io/photo/cherry-cherries-MYRKEUABZY"},
		{URL: "https://live.staticflickr.com/65535/51295471407_884e2f094d_b.jpg", License: "pdm", Page: "https://www.flickr.com/photos/190270151@N02/51295471407"},
	},
	"gisht": {
		{URL: "https://cdn.stocksnap.io/img-thumbs/960w/4YCB0VAY1L.jpg", License: "cc0", Page: "https://stocksnap.io/photo/bricks-wall-4YCB0VAY1L"},
		{URL: "https://cdn.stocksnap.io/img-thumbs/960w/G6BSNJW73Y.jpg", License: "cc0", Page: "https://stocksnap.io/photo/bricks-wall-G6BSNJW73Y"},
		{URL: "https://live.staticflickr.com/420/31712988076_b32517f391_b.jpg", License: "pdm", Page: "https://www.flickr.com/photos/145898134@N07/31712988076"},
	},
	"gulkaram": {
		{URL: "https://live.staticflickr.com/65535/51299735504_7389cf0fa1_b.jpg", License: "cc0", Page: "https://www.flickr.com/photos/184594136@N08/51299735504"},
		{URL: "https://live.staticflickr.com/65535/51299026616_2f6081553d_b.jpg", License: "cc0", Page: "https://www.flickr.com/photos/184594136@N08/51299026616"},
		{URL: "https://live.staticflickr.com/65535/52468040946_cf95164d4e_b.jpg", License: "pdm", Page: "https://www.flickr.com/photos/129016680@N06/52468040946"},
	},
	"guruch": {
		{URL: "https://images.rawpixel.com/editor_1024/cHJpdmF0ZS9zdGF0aWMvaW1hZ2Uvd2Vic2l0ZS8yMDIyLTA0L2xyL3B4MTYwMzUyNy1pbWFnZS1rd3Z4cWI0ay5qcGc.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/5922922/photo-image-public-domain-food-free"},
	},
	"ismaloq": {
		{URL: "https://images.rawpixel.com/editor_1024/czNmcy1wcml2YXRlL3Jhd3BpeGVsX2ltYWdlcy93ZWJzaXRlX2NvbnRlbnQvbHIvcHg1NzI2OTAtaW1hZ2Uta3d2dmRtMW0uanBn.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/5908036/image-public-domain-leaf-plant"},
		{URL: "https://live.staticflickr.com/65535/52865459377_793b24fd55_b.jpg", License: "cc0", Page: "https://www.flickr.com/photos/75126686@N00/52865459377"},
	},
	"joxori-doni": {
		{URL: "https://images.rawpixel.com/editor_1024/czNmcy1wcml2YXRlL3Jhd3BpeGVsX2ltYWdlcy93ZWJzaXRlX2NvbnRlbnQvbHIvZnJjb3JuX2NlcmVhbHNfaGFydmVzdF8xMTczODE2LWltYWdlLWt5YmR5cDQxLmpwZw.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/6039371/dried-corn-kernels-free-public-domain-cc0-photo"},
		{URL: "https://images.rawpixel.com/editor_1024/cHJpdmF0ZS9zdGF0aWMvaW1hZ2Uvd2Vic2l0ZS8yMDIyLTA0L2xyL3B4MTYwMzQ5Ni1pbWFnZS1rd3Z2bnJoai5qcGc.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/5909716/image-background-public-domain-golden"},
		{URL: "https://live.staticflickr.com/65535/51554928408_ba1dd2fed1_b.jpg", License: "cc0", Page: "https://www.flickr.com/photos/184594136@N08/51554928408"},
		{URL: "https://live.staticflickr.com/65535/51554928488_ffaceab2f9_b.jpg", License: "cc0", Page: "https://www.flickr.com/photos/184594136@N08/51554928488"},
	},
	"kabachok": {
		{URL: "https://live.staticflickr.com/65535/52232155936_28c549351b_b.jpg", License: "cc0", Page: "https://www.flickr.com/photos/184594136@N08/52232155936"},
		{URL: "https://live.staticflickr.com/65535/52232164288_5e03404de7_b.jpg", License: "cc0", Page: "https://www.flickr.com/photos/184594136@N08/52232164288"},
	},
	"kafel": {
		{URL: "https://images.rawpixel.com/editor_1024/cHJpdmF0ZS9zdGF0aWMvZmlsZXMvd2Vic2l0ZS8yMDIzLTExL21ldDE4NzkzNi1pbWFnZS5qcGc.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/12851675/pavement-tiles"},
	},
	"karam": {
		{URL: "https://live.staticflickr.com/65535/51299736854_7438f66ca3_b.jpg", License: "cc0", Page: "https://www.flickr.com/photos/184594136@N08/51299736854"},
	},
	"kartoshka": {
		{URL: "https://cdn.stocksnap.io/img-thumbs/960w/RYSFFCA1QV.jpg", License: "cc0", Page: "https://stocksnap.io/photo/potatoes-onions-RYSFFCA1QV"},
		{URL: "https://cdn.stocksnap.io/img-thumbs/960w/C7LDIUGZ9K.jpg", License: "cc0", Page: "https://stocksnap.io/photo/beans-farm-C7LDIUGZ9K"},
		{URL: "https://cdn.stocksnap.io/img-thumbs/960w/AWJD4WV6W1.jpg", License: "cc0", Page: "https://stocksnap.io/photo/food-fruits-AWJD4WV6W1"},
	},
	"kashnich": {
		{URL: "https://images.rawpixel.com/editor_1024/czNmcy1wcml2YXRlL3Jhd3BpeGVsX2ltYWdlcy93ZWJzaXRlX2NvbnRlbnQvbHIvZmw1MDU5NDg5Mzk0MS1pbWFnZS1reWNpb3huZi5qcGc.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/6074654/cilantro"},
		{URL: "https://live.staticflickr.com/65535/54687497827_cd2bb8fc37_b.jpg", License: "cc0", Page: "https://www.flickr.com/photos/37996646802@N01/54687497827"},
	},
	"kivi": {
		{URL: "https://images.rawpixel.com/editor_1024/cHJpdmF0ZS9sci9pbWFnZXMvd2Vic2l0ZS8yMDIyLTExL2ZsNTE2Njg2Nzg3NjAtaW1hZ2UuanBn.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/8731981/photo-image-public-domain-fruit"},
		{URL: "https://cdn.stocksnap.io/img-thumbs/960w/QLO68U090J.jpg", License: "cc0", Page: "https://stocksnap.io/photo/green-kiwi-QLO68U090J"},
		{URL: "https://images.rawpixel.com/editor_1024/czNmcy1wcml2YXRlL3Jhd3BpeGVsX2ltYWdlcy93ZWJzaXRlX2NvbnRlbnQvbHIvZmw0MDYyMDY5MzE0MS1pbWFnZS1reWNqNWZndS5qcGc.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/6077703/kiwi-fruit"},
	},
	"kok-piyoz": {
		{URL: "https://images.rawpixel.com/editor_1024/cHJpdmF0ZS9zdGF0aWMvaW1hZ2Uvd2Vic2l0ZS8yMDIyLTA0L2xyL2ZyYWxsaXVtX2J1bmNoX2NlcGFfZ3JlZW5fMC1pbWFnZS1reWJlY2lqNS5qcGc.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/6041949/photo-image-public-domain-green-food"},
		{URL: "https://images.rawpixel.com/editor_1024/cHJpdmF0ZS9zdGF0aWMvaW1hZ2Uvd2Vic2l0ZS8yMDIyLTA0L2xyL3B4NjE3MDExLWltYWdlLWt3dnhteGNsLmpwZw.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/5922219/photo-image-public-domain-free-vegetable"},
		{URL: "https://images.rawpixel.com/editor_1024/cHJpdmF0ZS9zdGF0aWMvaW1hZ2Uvd2Vic2l0ZS8yMDIyLTA0L2xyL2Zyc3ByaW5nX29uaW9uc19sZWVrX2RlbGljaW91c18yLWltYWdlLWt5YmFsejAxLmpwZw.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/6018169/photo-image-public-domain-green-food"},
		{URL: "https://images.rawpixel.com/editor_1024/czNmcy1wcml2YXRlL3Jhd3BpeGVsX2ltYWdlcy93ZWJzaXRlX2NvbnRlbnQvbHIvaXMxMDg2OC1pbWFnZS1rd3lzaHlxdi5qcGc.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/5970803/spring-onions"},
	},
	"lavlagi": {
		{URL: "https://cdn.stocksnap.io/img-thumbs/960w/EWTPWWQBRB.jpg", License: "cc0", Page: "https://stocksnap.io/photo/vegetables-food-EWTPWWQBRB"},
		{URL: "https://cdn.stocksnap.io/img-thumbs/960w/XCYDMSCHW5.jpg", License: "cc0", Page: "https://stocksnap.io/photo/vegetables-food-XCYDMSCHW5"},
	},
	"limon": {
		{URL: "https://live.staticflickr.com/65535/51427721961_1e151ec92f_b.jpg", License: "cc0", Page: "https://www.flickr.com/photos/40632439@N00/51427721961"},
		{URL: "https://cdn.stocksnap.io/img-thumbs/960w/4CIBHHJOHE.jpg", License: "cc0", Page: "https://stocksnap.io/photo/yellow-lemons-4CIBHHJOHE"},
		{URL: "https://cdn.stocksnap.io/img-thumbs/960w/W28QPZPAK6.jpg", License: "cc0", Page: "https://stocksnap.io/photo/lemons-fruits-W28QPZPAK6"},
	},
	"loviya": {
		{URL: "https://images.rawpixel.com/editor_1024/czNmcy1wcml2YXRlL3Jhd3BpeGVsX2ltYWdlcy93ZWJzaXRlX2NvbnRlbnQvcHgxMTA1NzQyLWltYWdlLWt3dndhbG01LmpwZw.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/5913548/image-public-domain-plant-red"},
		{URL: "https://images.rawpixel.com/editor_1024/czNmcy1wcml2YXRlL3Jhd3BpeGVsX2ltYWdlcy93ZWJzaXRlX2NvbnRlbnQvbHIvcHgxNjAzNTMyLWltYWdlLWt3dnhyOGVxLmpwZw.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/5923170/photo-image-background-public-domain-plant"},
		{URL: "https://images.rawpixel.com/editor_1024/czNmcy1wcml2YXRlL3Jhd3BpeGVsX2ltYWdlcy93ZWJzaXRlX2NvbnRlbnQvbHIvcHg3NjU5NjEtaW1hZ2Uta3d2eGZ3M3cuanBn.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/5921123/photo-image-public-domain-plant-hand"},
		{URL: "https://images.rawpixel.com/editor_1024/czNmcy1wcml2YXRlL3Jhd3BpeGVsX2ltYWdlcy93ZWJzaXRlX2NvbnRlbnQvbHIvcHgxMzY1ODk1LWltYWdlLWt3dncyOXZoLmpwZw.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/5912241/image-public-domain-plant-hand"},
	},
	"makaron": {
		{URL: "https://cdn.stocksnap.io/img-thumbs/960w/A06F9A9896.jpg", License: "cc0", Page: "https://stocksnap.io/photo/spaghetti-pasta-A06F9A9896"},
	},
	"makkajoxori": {
		{URL: "https://images.rawpixel.com/editor_1024/cHJpdmF0ZS9zdGF0aWMvaW1hZ2Uvd2Vic2l0ZS8yMDIyLTA0L2xyL3B4MTExODIyMi1pbWFnZS1rd3Z3OTV2by5qcGc.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/5913428/image-public-domain-leaves-free"},
		{URL: "https://images.rawpixel.com/editor_1024/czNmcy1wcml2YXRlL3Jhd3BpeGVsX2ltYWdlcy93ZWJzaXRlX2NvbnRlbnQvbHIvcHg3MjQxNjktaW1hZ2Uta3d2eGlmMXEuanBn.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/5921736/free-fresh-corn-cob-the-pot"},
		{URL: "https://images.rawpixel.com/editor_1024/cHJpdmF0ZS9zdGF0aWMvaW1hZ2Uvd2Vic2l0ZS8yMDIyLTA0L2xyL3B4MTU5MjgyNC1pbWFnZS1rd3Z2cDZieC5qcGc.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/5909973/image-public-domain-free-vegetables"},
		{URL: "https://images.rawpixel.com/editor_1024/czNmcy1wcml2YXRlL3Jhd3BpeGVsX2ltYWdlcy93ZWJzaXRlX2NvbnRlbnQvbHIvcHg1NDA1MDEtaW1hZ2Uta3d2eG96dnouanBn.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/5922585/photo-image-public-domain-leaves-free"},
	},
	"malina": {
		{URL: "https://cdn.stocksnap.io/img-thumbs/960w/HUBPTR7YNE.jpg", License: "cc0", Page: "https://stocksnap.io/photo/raspberry-cake-HUBPTR7YNE"},
		{URL: "https://cdn.stocksnap.io/img-thumbs/960w/XCIJ0QVBK8.jpg", License: "cc0", Page: "https://stocksnap.io/photo/raspberries-berries-XCIJ0QVBK8"},
		{URL: "https://cdn.stocksnap.io/img-thumbs/960w/8NHBORBRVR.jpg", License: "cc0", Page: "https://stocksnap.io/photo/raspberries-berries-8NHBORBRVR"},
	},
	"mandarin": {
		{URL: "https://live.staticflickr.com/65535/55101008087_3ceaf32ab4_b.jpg", License: "pdm", Page: "https://www.flickr.com/photos/25802865@N08/55101008087"},
		{URL: "https://live.staticflickr.com/65535/55154147480_6dee7338c6_b.jpg", License: "pdm", Page: "https://www.flickr.com/photos/25802865@N08/55154147480"},
		{URL: "https://live.staticflickr.com/65535/55091226703_c02ea5aa7b_b.jpg", License: "pdm", Page: "https://www.flickr.com/photos/25802865@N08/55091226703"},
	},
	"mayiz": {
		{URL: "https://live.staticflickr.com/65535/54824202350_cceb5880e5_b.jpg", License: "pdm", Page: "https://www.flickr.com/photos/25802865@N08/54824202350"},
	},
	"mol-goshti": {
		{URL: "https://images.rawpixel.com/editor_1024/czNmcy1wcml2YXRlL3Jhd3BpeGVsX2ltYWdlcy93ZWJzaXRlX2NvbnRlbnQvbHIvdXB3azYxNjY1Mjc3LXdpa2ltZWRpYS1pbWFnZS1rb3dsd3lncS5qcGc.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/3294344/free-photo-image-butcher-shop-cc0-creative-commons"},
		{URL: "https://images.rawpixel.com/editor_1024/czNmcy1wcml2YXRlL3Jhd3BpeGVsX2ltYWdlcy93ZWJzaXRlX2NvbnRlbnQvbHIvZmYyOTYzLWltYWdlLWt3dnluM2poLmpwZw.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/5928224/photo-image-public-domain-kitchen-food"},
	},
	"mosh": {
		{URL: "https://images.rawpixel.com/editor_1024/cHJpdmF0ZS9zdGF0aWMvaW1hZ2Uvd2Vic2l0ZS8yMDIyLTA0L2xyL3B4MTAyMjEwMi1pbWFnZS1rd3Z3Z3NtdS5qcGc.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/5914832/image-public-domain-plant-green"},
	},
	"nohat": {
		{URL: "https://images.rawpixel.com/editor_1024/czNmcy1wcml2YXRlL3Jhd3BpeGVsX2ltYWdlcy93ZWJzaXRlX2NvbnRlbnQvcHgxMTA1NzM3LWltYWdlLWt3dndheDF5LmpwZw.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/5913543/image-public-domain-food-white"},
	},
	"nok": {
		{URL: "https://cdn.stocksnap.io/img-thumbs/960w/GS6DRGU9MB.jpg", License: "cc0", Page: "https://stocksnap.io/photo/green-pears-GS6DRGU9MB"},
	},
	"non": {
		{URL: "https://cdn.stocksnap.io/img-thumbs/960w/WOBV6IVUD0.jpg", License: "cc0", Page: "https://stocksnap.io/photo/homemade-bread-WOBV6IVUD0"},
		{URL: "https://live.staticflickr.com/65535/55147622682_1846423e2f_b.jpg", License: "cc0", Page: "https://www.flickr.com/photos/96541566@N06/55147622682"},
	},
	"olcha": {
		{URL: "https://images.rawpixel.com/editor_1024/czNmcy1wcml2YXRlL3Jhd3BpeGVsX2ltYWdlcy93ZWJzaXRlX2NvbnRlbnQvbHIvZnJhbmRfY2hlcnJpZXNfcmVkX2ZydWl0LWltYWdlLWt5Y2liaXp5LmpwZw.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/6072111/free-public-domain-cc0-photo"},
		{URL: "https://images.rawpixel.com/editor_1024/czNmcy1wcml2YXRlL3Jhd3BpeGVsX2ltYWdlcy93ZWJzaXRlX2NvbnRlbnQvbHIvZnJzb3VyX2NoZXJyeV9tZWdneWVrX2ZydWl0LWltYWdlLWt5Y2ZnNjNxLmpwZw.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/6050023/free-public-domain-cc0-photo"},
	},
	"olma": {
		{URL: "https://images.rawpixel.com/editor_1024/czNmcy1wcml2YXRlL3Jhd3BpeGVsX2ltYWdlcy93ZWJzaXRlX2NvbnRlbnQvbHIvZnJhcHBsZXNfZnJ1aXRfcmVkX2FwcGxlXzEtaW1hZ2Uta3liZWNkOTguanBn.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/6041947/photo-image-public-domain-fruit-red"},
		{URL: "https://images.rawpixel.com/editor_1024/cHJpdmF0ZS9zdGF0aWMvaW1hZ2Uvd2Vic2l0ZS8yMDIyLTA0L2xyL2ZyYXBwbGVfYmFza2V0X2ZydWl0X2Jhc2tldF8wLWltYWdlLWt5YmM5ZXFyLmpwZw.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/6029393/photo-image-public-domain-fruit-red"},
		{URL: "https://images.rawpixel.com/editor_1024/cHJpdmF0ZS9zdGF0aWMvaW1hZ2Uvd2Vic2l0ZS8yMDIyLTA0L2xyL2ZyYXBwbGVfYmlvX2FwcGxlX25hdHVyYWwtaW1hZ2Uta3liY2FqaTguanBn.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/6029465/photo-image-public-domain-fruit-red"},
		{URL: "https://images.rawpixel.com/editor_1024/cHJpdmF0ZS9zdGF0aWMvaW1hZ2Uvd2Vic2l0ZS8yMDIyLTA0L2xyL2ZyYXBwbGVfY3V0dGluZ19ib2FyZF9iYXNrZXRfMC1pbWFnZS1reWJlZGxhbC5qcGc.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/6041910/photo-image-public-domain-fruit-red"},
	},
	"olxori": {
		{URL: "https://images.rawpixel.com/editor_1024/cHJpdmF0ZS9zdGF0aWMvaW1hZ2Uvd2Vic2l0ZS8yMDIyLTA0L2xyL2ZyZnJ1aXRfcGx1bXNfYnJhbmNoX3BsdW0taW1hZ2Uta3liZG5tYTQuanBn.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/6037376/photo-image-public-domain-leaf-tree"},
		{URL: "https://images.rawpixel.com/editor_1024/cHJpdmF0ZS9zdGF0aWMvaW1hZ2Uvd2Vic2l0ZS8yMDIyLTA0L2xyL2ZycGx1bXNfcGx1bV90cmVlX2ZydWl0XzEtaW1hZ2Uta3liY3gxNnQuanBn.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/6033153/photo-image-public-domain-leaf-tree"},
		{URL: "https://images.rawpixel.com/editor_1024/czNmcy1wcml2YXRlL3Jhd3BpeGVsX2ltYWdlcy93ZWJzaXRlX2NvbnRlbnQvbHIvZnJwbHVtc19wbHVtX3RyZWVfZnJ1aXRfNy1pbWFnZS1reWJjd21tNS5qcGc.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/6033222/photo-image-public-domain-leaf-tree"},
		{URL: "https://images.rawpixel.com/editor_1024/cHJpdmF0ZS9zdGF0aWMvaW1hZ2Uvd2Vic2l0ZS8yMDIyLTA0L2xyL2ZycGx1bV9mcnVpdF9yZWRfcGx1bV8wLWltYWdlLWt5YmF1N290LmpwZw.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/6019721/pile-plum-fruit-free-public-domain-cc0-photo"},
	},
	"ombor-ijarasi": {
		{URL: "https://images.rawpixel.com/editor_1024/czNmcy1wcml2YXRlL3Jhd3BpeGVsX2ltYWdlcy93ZWJzaXRlX2NvbnRlbnQvbHIvZnJsb2dpc3RpY3NfODUyOTM3LWltYWdlLWt5Y2dmZGM0LmpwZw.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/6057556/free-public-domain-cc0-photo"},
		{URL: "https://images.rawpixel.com/editor_1024/cHJpdmF0ZS9sci9pbWFnZXMvd2Vic2l0ZS8yMDIyLTA0L3Vwd2s2MjA3MDM2NS13aWtpbWVkaWEtaW1hZ2Uta293bzA4MjguanBn.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/3298148/free-photo-image-warehouse-aisle-book"},
	},
	"orik": {
		{URL: "https://images.rawpixel.com/editor_1024/cHJpdmF0ZS9zdGF0aWMvaW1hZ2Uvd2Vic2l0ZS8yMDIyLTA0L2xyL2ZyYXByaWNvdHNfZnJ1aXRfb3JhbmdlX3BpcHNfMi1pbWFnZS1reWJjYjBqMC5qcGc.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/6029631/apricots-free-public-domain-cc0-image"},
		{URL: "https://images.rawpixel.com/editor_1024/cHJpdmF0ZS9zdGF0aWMvaW1hZ2Uvd2Vic2l0ZS8yMDIyLTA0L2xyL3B4ODc1MjM1LWltYWdlLWt3dnhlMTNwLmpwZw.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/5920675/photo-image-public-domain-tree-plant"},
		{URL: "https://images.rawpixel.com/editor_1024/czNmcy1wcml2YXRlL3Jhd3BpeGVsX2ltYWdlcy93ZWJzaXRlX2NvbnRlbnQvbHIvZnJhcHJpY290X2hhcnZlc3RfZnJ1aXRfdHJlZS1pbWFnZS1reWNpY2JxZy5qcGc.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/6072261/free-public-domain-cc0-photo"},
	},
	"ot-goshti": {
		{URL: "https://cdn.stocksnap.io/img-thumbs/960w/N2A2PQLW4D.jpg", License: "cc0", Page: "https://stocksnap.io/photo/food-cook-N2A2PQLW4D"},
		{URL: "https://images.rawpixel.com/editor_1024/cHJpdmF0ZS9zdGF0aWMvaW1hZ2Uvd2Vic2l0ZS8yMDIyLTA0L2xyL2Zyc2lybG9pbl9veF9iZWVmX3JvYXN0ZWQtaW1hZ2Uta3liY3FrYTUuanBn.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/6032130/red-raw-meat-free-public-domain-cc0-photo"},
		{URL: "https://images.rawpixel.com/editor_1024/czNmcy1wcml2YXRlL3Jhd3BpeGVsX2ltYWdlcy93ZWJzaXRlX2NvbnRlbnQvbHIvaXMxNjA4Mi1pbWFnZS1rd3Z5Z24xcC5qcGc.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/5927080/photo-image-public-domain-food-free"},
		{URL: "https://images.rawpixel.com/editor_1024/czNmcy1wcml2YXRlL3Jhd3BpeGVsX2ltYWdlcy93ZWJzaXRlX2NvbnRlbnQvbHIvZnJzdGVha19wb3J0ZXJob3VzZV9iYXJiZWN1ZV8xMTM4NTYzLWltYWdlLWt5YmNxOWU0LmpwZw.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/6031986/grilling-raw-meat-free-public-domain-cc0-photo"},
	},
	"petrushka": {
		{URL: "https://cdn.stocksnap.io/img-thumbs/960w/XVIICAUSKR.jpg", License: "cc0", Page: "https://stocksnap.io/photo/herbs-parsley-XVIICAUSKR"},
	},
	"pishloq": {
		{URL: "https://cdn.stocksnap.io/img-thumbs/960w/ABFRSZL8XB.jpg", License: "cc0", Page: "https://stocksnap.io/photo/cheese-food-ABFRSZL8XB"},
		{URL: "https://live.staticflickr.com/65535/51421814670_1de24423dc_b.jpg", License: "cc0", Page: "https://www.flickr.com/photos/96218136@N08/51421814670"},
	},
	"pista": {
		{URL: "https://cdn.stocksnap.io/img-thumbs/960w/D3JX2C0MW9.jpg", License: "cc0", Page: "https://stocksnap.io/photo/pistachios-nuts-D3JX2C0MW9"},
		{URL: "https://cdn.stocksnap.io/img-thumbs/960w/ZH38X52WOT.jpg", License: "cc0", Page: "https://stocksnap.io/photo/pistachios-nuts-ZH38X52WOT"},
		{URL: "https://live.staticflickr.com/65535/55333290458_e477e19182_b.jpg", License: "pdm", Page: "https://www.flickr.com/photos/25802865@N08/55333290458"},
		{URL: "https://cdn.stocksnap.io/img-thumbs/960w/XFTX0Y1LUK.jpg", License: "cc0", Page: "https://stocksnap.io/photo/pistachio-wood-XFTX0Y1LUK"},
	},
	"piyoz": {
		{URL: "https://cdn.stocksnap.io/img-thumbs/960w/56SUTM56T5.jpg", License: "cc0", Page: "https://stocksnap.io/photo/onions-garlic-56SUTM56T5"},
		{URL: "https://cdn.stocksnap.io/img-thumbs/960w/8OY1EZZVXX.jpg", License: "cc0", Page: "https://stocksnap.io/photo/onions-food-8OY1EZZVXX"},
	},
	"pomidor": {
		{URL: "https://cdn.stocksnap.io/img-thumbs/960w/GB9LU1L8RG.jpg", License: "cc0", Page: "https://stocksnap.io/photo/red-tomatoes-GB9LU1L8RG"},
		{URL: "https://live.staticflickr.com/65535/51986998271_51385aef28_b.jpg", License: "pdm", Page: "https://www.flickr.com/photos/44636446@N04/51986998271"},
		{URL: "https://live.staticflickr.com/65535/51859296005_6434368926_b.jpg", License: "cc0", Page: "https://www.flickr.com/photos/184594136@N08/51859296005"},
	},
	"qadoqlash": {
		{URL: "https://cdn.stocksnap.io/img-thumbs/960w/USP4WCYPBW.jpg", License: "cc0", Page: "https://stocksnap.io/photo/cardboard-box-USP4WCYPBW"},
		{URL: "https://cdn.stocksnap.io/img-thumbs/960w/PIHUS3UFDJ.jpg", License: "cc0", Page: "https://stocksnap.io/photo/gift-box-PIHUS3UFDJ"},
	},
	"qatiq": {
		{URL: "https://cdn.stocksnap.io/img-thumbs/960w/EE782F841E.jpg", License: "cc0", Page: "https://stocksnap.io/photo/parfait-yogurt-EE782F841E"},
		{URL: "https://live.staticflickr.com/65535/52234483098_cdd69ba12d_b.jpg", License: "cc0", Page: "https://www.flickr.com/photos/184594136@N08/52234483098"},
	},
	"qazi": {
		{URL: "https://live.staticflickr.com/65535/55079225675_254f133998_b.jpg", License: "pdm", Page: "https://www.flickr.com/photos/25802865@N08/55079225675"},
	},
	"qiyma": {
		{URL: "https://images.rawpixel.com/editor_1024/czNmcy1wcml2YXRlL3Jhd3BpeGVsX2ltYWdlcy93ZWJzaXRlX2NvbnRlbnQvbHIvZnJtaW5jZXJfbWluY2VkX21lYXRfZ3JvdW5kXzAtaW1hZ2Uta3liZDV6dW0uanBn.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/6034641/ground-raw-meat-free-public-domain-cc0-photo"},
	},
	"qovoq": {
		{URL: "https://live.staticflickr.com/65535/51560202269_68297e72d6_b.jpg", License: "cc0", Page: "https://www.flickr.com/photos/47121680@N00/51560202269"},
		{URL: "https://live.staticflickr.com/65535/52451262493_4f27e217f8_b.jpg", License: "cc0", Page: "https://www.flickr.com/photos/184594136@N08/52451262493"},
	},
	"qovun": {
		{URL: "https://images.rawpixel.com/editor_1024/cHJpdmF0ZS9zdGF0aWMvaW1hZ2Uvd2Vic2l0ZS8yMDIyLTA0L2xyL2ZybWVsb25zX3F1b3RfY2hhcmVudGFpc19xdW90LWltYWdlLWt5YmIyaWF5LmpwZw.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/6021359/photo-image-public-domain-free"},
		{URL: "https://live.staticflickr.com/65535/52307009259_d89884878b_b.jpg", License: "cc0", Page: "https://www.flickr.com/photos/184594136@N08/52307009259"},
	},
	"qoy-goshti": {
		{URL: "https://images.rawpixel.com/editor_1024/czNmcy1wcml2YXRlL3Jhd3BpeGVsX2ltYWdlcy93ZWJzaXRlX2NvbnRlbnQvbHIvZnJidXRjaGVyX3NsYXVnaHRlcl9tZWF0X2Zvb2RfMC1pbWFnZS1reWJlNTc2Yi5qcGc.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/6040551/raw-meat-free-public-domain-cc0-photo"},
	},
	"qulupnay": {
		{URL: "https://cdn.stocksnap.io/img-thumbs/960w/PROXEBKRSP.jpg", License: "cc0", Page: "https://stocksnap.io/photo/strawberry-food-PROXEBKRSP"},
		{URL: "https://cdn.stocksnap.io/img-thumbs/960w/VUTKKYY1YD.jpg", License: "cc0", Page: "https://stocksnap.io/photo/red-strawberries-VUTKKYY1YD"},
	},
	"rayhon": {
		{URL: "https://live.staticflickr.com/65535/51859295900_141eb0ba13_b.jpg", License: "cc0", Page: "https://www.flickr.com/photos/184594136@N08/51859295900"},
	},
	"sabzi": {
		{URL: "https://cdn.stocksnap.io/img-thumbs/960w/RKHZEX3629.jpg", License: "cc0", Page: "https://stocksnap.io/photo/carrots-vegetables-RKHZEX3629"},
		{URL: "https://cdn.stocksnap.io/img-thumbs/960w/VXT0GH53QR.jpg", License: "cc0", Page: "https://stocksnap.io/photo/carrot-vegetables-VXT0GH53QR"},
		{URL: "https://cdn.stocksnap.io/img-thumbs/960w/VD76H3TKVJ.jpg", License: "cc0", Page: "https://stocksnap.io/photo/food-carrots-VD76H3TKVJ"},
	},
	"salat-bargi": {
		{URL: "https://cdn.stocksnap.io/img-thumbs/960w/WLZ7N1WO0I.jpg", License: "cc0", Page: "https://stocksnap.io/photo/green-lettuce-WLZ7N1WO0I"},
		{URL: "https://cdn.stocksnap.io/img-thumbs/960w/LPL5U4N1TJ.jpg", License: "cc0", Page: "https://stocksnap.io/photo/salad-lettuce-LPL5U4N1TJ"},
		{URL: "https://live.staticflickr.com/65535/51857678832_888b7749aa_b.jpg", License: "cc0", Page: "https://www.flickr.com/photos/184594136@N08/51857678832"},
	},
	"sarimsoq": {
		{URL: "https://cdn.stocksnap.io/img-thumbs/960w/17E04668C5.jpg", License: "cc0", Page: "https://stocksnap.io/photo/garlic-cloves-17E04668C5"},
		{URL: "https://cdn.stocksnap.io/img-thumbs/960w/2QSYLT07Q0.jpg", License: "cc0", Page: "https://stocksnap.io/photo/garlic-cloves-2QSYLT07Q0"},
		{URL: "https://cdn.stocksnap.io/img-thumbs/960w/AGRMVP0ADI.jpg", License: "cc0", Page: "https://stocksnap.io/photo/garlic-spice-AGRMVP0ADI"},
	},
	"shaftoli": {
		{URL: "https://live.staticflickr.com/8352/29191981444_9b3b6f4119_b.jpg", License: "pdm", Page: "https://www.flickr.com/photos/145331800@N02/29191981444"},
		{URL: "https://cdn.stocksnap.io/img-thumbs/960w/C8VA0WIM0R.jpg", License: "cc0", Page: "https://stocksnap.io/photo/peach-fruit-C8VA0WIM0R"},
	},
	"shagal": {
		{URL: "https://live.staticflickr.com/7054/27846258795_769834acce.jpg", License: "pdm", Page: "https://www.flickr.com/photos/38411472@N03/27846258795"},
	},
	"shakar": {
		{URL: "https://images.rawpixel.com/editor_1024/czNmcy1wcml2YXRlL3Jhd3BpeGVsX2ltYWdlcy93ZWJzaXRlX2NvbnRlbnQvbHIvZnJzZW1vbGluYV9mbG91cl9ncmFudWxlX3doZWF0XzUtaW1hZ2Uta3ljZmtxZ2QuanBn.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/6050809/free-public-domain-cc0-photo"},
	},
	"shivit": {
		{URL: "https://images.rawpixel.com/editor_1024/czNmcy1wcml2YXRlL3Jhd3BpeGVsX2ltYWdlcy93ZWJzaXRlX2NvbnRlbnQvbHIvZmYzODIwLWltYWdlLWt3dngyeWJvLmpwZw.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/5919011/image-public-domain-plant-food"},
	},
	"sholgom": {
		{URL: "https://live.staticflickr.com/65535/51295471377_8ca594ebe0_b.jpg", License: "pdm", Page: "https://www.flickr.com/photos/190270151@N02/51295471377"},
	},
	"suli": {
		{URL: "https://images.rawpixel.com/editor_1024/czNmcy1wcml2YXRlL3Jhd3BpeGVsX2ltYWdlcy93ZWJzaXRlX2NvbnRlbnQvbHIvcHg3NDQ5NjEtaW1hZ2Uta3d2eGZzaTcuanBn.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/5921094/free-public-domain-cc0-photo"},
	},
	"sut": {
		{URL: "https://cdn.stocksnap.io/img-thumbs/960w/P2QSUXKCN5.jpg", License: "cc0", Page: "https://stocksnap.io/photo/glass-milk-P2QSUXKCN5"},
		{URL: "https://live.staticflickr.com/65535/52633675811_b6b70f29c8_b.jpg", License: "cc0", Page: "https://www.flickr.com/photos/184594136@N08/52633675811"},
		{URL: "https://live.staticflickr.com/65535/52633675771_652c829053_b.jpg", License: "cc0", Page: "https://www.flickr.com/photos/184594136@N08/52633675771"},
	},
	"tarvuz": {
		{URL: "https://cdn.stocksnap.io/img-thumbs/960w/GDTA8DSYQ7.jpg", License: "cc0", Page: "https://stocksnap.io/photo/watermelon-fruits-GDTA8DSYQ7"},
		{URL: "https://cdn.stocksnap.io/img-thumbs/960w/30C9EB71DC.jpg", License: "cc0", Page: "https://stocksnap.io/photo/watermelon-fruits-30C9EB71DC"},
		{URL: "https://cdn.stocksnap.io/img-thumbs/960w/GC2VG8XM7O.jpg", License: "cc0", Page: "https://stocksnap.io/photo/watermelon-fruits-GC2VG8XM7O"},
	},
	"taxta": {
		{URL: "https://cdn.stocksnap.io/img-thumbs/960w/SSIXAR7YJY.jpg", License: "cc0", Page: "https://stocksnap.io/photo/wood-texture-SSIXAR7YJY"},
		{URL: "https://cdn.stocksnap.io/img-thumbs/960w/BJWPE8ABOZ.jpg", License: "cc0", Page: "https://stocksnap.io/photo/wood-texture-BJWPE8ABOZ"},
		{URL: "https://cdn.stocksnap.io/img-thumbs/960w/BJ0QEAJWUM.jpg", License: "cc0", Page: "https://stocksnap.io/photo/wood-texture-BJ0QEAJWUM"},
	},
	"tomat-pastasi": {
		{URL: "https://images.rawpixel.com/editor_1024/cHJpdmF0ZS9sci9pbWFnZXMvd2Vic2l0ZS8yMDIyLTA0L3Vwd2s2MTY2NDAzMS13aWtpbWVkaWEtaW1hZ2Uta293cjdncDguanBn.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/3304068/free-photo-image-tomato-juice-cc0"},
	},
	"turp": {
		{URL: "https://live.staticflickr.com/65535/54730343759_c971ecd2b4_b.jpg", License: "cc0", Page: "https://www.flickr.com/photos/40632439@N00/54730343759"},
		{URL: "https://cdn.stocksnap.io/img-thumbs/960w/AYSFS91ZMX.jpg", License: "cc0", Page: "https://stocksnap.io/photo/radish-vegetables-AYSFS91ZMX"},
		{URL: "https://cdn.stocksnap.io/img-thumbs/960w/D7UIHFR8LI.jpg", License: "cc0", Page: "https://stocksnap.io/photo/radish-vegetables-D7UIHFR8LI"},
	},
	"tuxum": {
		{URL: "https://cdn.stocksnap.io/img-thumbs/960w/MGWWJDK49D.jpg", License: "cc0", Page: "https://stocksnap.io/photo/brown-eggs-MGWWJDK49D"},
		{URL: "https://images.rawpixel.com/editor_1024/czNmcy1wcml2YXRlL3Jhd3BpeGVsX2ltYWdlcy93ZWJzaXRlX2NvbnRlbnQvbHIvdXB3azYyMDUyNzEwLXdpa2ltZWRpYS1pbWFnZS1rb3dqZ2piYy5qcGc.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/3289792/free-photo-image-baking-bowl-breakfast"},
		{URL: "https://images.rawpixel.com/editor_1024/czNmcy1wcml2YXRlL3Jhd3BpeGVsX2ltYWdlcy93ZWJzaXRlX2NvbnRlbnQvbHIvdXB3azYyMDk5NzI3LXdpa2ltZWRpYS1pbWFnZS1rb3dnM3k1bC5qcGc.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/3286595/free-photo-image-egg-cooking"},
	},
	"tuz": {
		{URL: "https://cdn.stocksnap.io/img-thumbs/960w/6HO3GQMD6J.jpg", License: "cc0", Page: "https://stocksnap.io/photo/macro-salt-6HO3GQMD6J"},
	},
	"tvorog": {
		{URL: "https://live.staticflickr.com/65535/51431445184_66ac37ff85_b.jpg", License: "pdm", Page: "https://www.flickr.com/photos/41284017@N08/51431445184"},
		{URL: "https://live.staticflickr.com/65535/51430940033_a50c6b12c9_b.jpg", License: "pdm", Page: "https://www.flickr.com/photos/41284017@N08/51430940033"},
	},
	"un": {
		{URL: "https://images.rawpixel.com/editor_1024/cHJpdmF0ZS9sci9pbWFnZXMvd2Vic2l0ZS8yMDIyLTA0L3Vwd2s2MTg0Njk4Ni13aWtpbWVkaWEtaW1hZ2Uta293cGpxZWQuanBn.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/3300788/free-photo-image-colombia-flour-sack-coffee"},
		{URL: "https://cdn.stocksnap.io/img-thumbs/960w/HOKEB8G6JM.jpg", License: "cc0", Page: "https://stocksnap.io/photo/baking-flour-HOKEB8G6JM"},
	},
	"uzum": {
		{URL: "https://live.staticflickr.com/65535/51424346135_d0835494c7_b.jpg", License: "pdm", Page: "https://www.flickr.com/photos/82995800@N06/51424346135"},
	},
	"xurmo": {
		{URL: "https://live.staticflickr.com/65535/55062895786_e2e23954e8_b.jpg", License: "pdm", Page: "https://www.flickr.com/photos/25802865@N08/55062895786"},
		{URL: "https://live.staticflickr.com/65535/52137522772_5aa134a9c3_b.jpg", License: "pdm", Page: "https://www.flickr.com/photos/77874836@N04/52137522772"},
	},
	"yasmiq": {
		{URL: "https://live.staticflickr.com/4794/40800367721_6af844a1d2_b.jpg", License: "cc0", Page: "https://www.flickr.com/photos/134468672@N06/40800367721"},
	},
	"yongoq": {
		{URL: "https://cdn.stocksnap.io/img-thumbs/960w/KQK5TLH6E1.jpg", License: "cc0", Page: "https://stocksnap.io/photo/walnuts-nuts-KQK5TLH6E1"},
		{URL: "https://live.staticflickr.com/7205/27560190240_147dec020f_b.jpg", License: "pdm", Page: "https://www.flickr.com/photos/111661024@N07/27560190240"},
	},
	"zanjabil": {
		{URL: "https://images.rawpixel.com/editor_1024/czNmcy1wcml2YXRlL3Jhd3BpeGVsX2ltYWdlcy93ZWJzaXRlX2NvbnRlbnQvbHIvcHgxMDg0MjIzLWltYWdlLWt3dndjbHZqLmpwZw.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/5913937/image-public-domain-shadow-nature"},
		{URL: "https://images.rawpixel.com/editor_1024/czNmcy1wcml2YXRlL3Jhd3BpeGVsX2ltYWdlcy93ZWJzaXRlX2NvbnRlbnQvbHIvcHgxMzQ5MjA3LWltYWdlLWt3dnh4OXM5LmpwZw.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/5923941/photo-image-public-domain-food-free"},
		{URL: "https://images.rawpixel.com/editor_1024/cHJpdmF0ZS9zdGF0aWMvaW1hZ2Uvd2Vic2l0ZS8yMDIyLTA0L2xyL3B4NjI1NTA3LWltYWdlLWt3dnZjZTJnLmpwZw.jpg", License: "cc0", Page: "https://www.rawpixel.com/image/5907789/photo-image-public-domain-food-free"},
	},
}
