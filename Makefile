.PHONY: build run test lint clean help

# 默认目标
all: build

# 编译
build:
	@echo "正在编译..."
	go build -o bin/fundmind ./cmd

# 运行
run:
	@echo "启动服务..."
	go run ./cmd

# 运行测试
test:
	@echo "运行测试..."
	go test -v ./...

# 代码检查
lint:
	@echo "代码检查..."
	go vet ./...
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint 未安装，跳过"; \
	fi

# 清理构建产物
clean:
	@echo "清理..."
	rm -rf bin/

# 下载依赖
deps:
	@echo "下载依赖..."
	go mod tidy
	go mod download

# 帮助
help:
	@echo "FundMind 开发命令"
	@echo ""
	@echo "使用方法:"
	@echo "  make build   - 编译项目"
	@echo "  make run     - 运行服务"
	@echo "  make test    - 运行测试"
	@echo "  make lint    - 代码检查"
	@echo "  make clean   - 清理构建产物"
	@echo "  make deps    - 下载依赖"
