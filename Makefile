.PHONY: test testdata testdata-ome lint

test:
	go test ./...

testdata:
	mkdir -p testdata
	curl -L https://static.webknossos.org/data/zarr_v3/l4_sample.zip -o testdata/l4_sample.zip
	cd testdata && unzip -o l4_sample.zip

testdata-ome:
	mkdir -p testdata/ome/v0.6
	if [ ! -d testdata/ome/v0.6/examples/.git ]; then \
		git clone --depth 1 https://github.com/jo-mueller/ngff-rfc5-coordinate-transformation-examples.git testdata/ome/v0.6/examples; \
	fi

lint:
	go vet ./...
	gofmt -l .
