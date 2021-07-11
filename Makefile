mod-fix:
	git clone https://github.com/jay-dee7/go-skynet.git /root/go-skynet
	go mod edit -replace="github.com/NebulousLabs/go-skynet/v2@v2.0.1=/root/go-skynet/"
