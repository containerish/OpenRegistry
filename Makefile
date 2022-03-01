mod-fix:
	git clone https://github.com/jay-dee7/go-skynet.git .go-skynet

tools:
	pip3 install ggshield pre-commit
	pre-commit install

certs:
	bash scripts/localcerts.sh
