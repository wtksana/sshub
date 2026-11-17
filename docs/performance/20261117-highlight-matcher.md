# 自定义日志规则：匹配与交付微基准

2026-11-17，Windows amd64，Go1.27.4，Intel Core Ultra X7 358H。以下为默认优化的 Go benchmark，非终端渲染性能；不代表最高吞吐、帧率或屏幕交互延迟。

命令：

```
go test -tags mygo_nowebview ./internal/connection -run '^$' -bench BenchmarkHighlightMatcher -benchmem -benchtime=1s -count=1
go test -tags mygo_nowebview ./internal/connection -run '^$' -bench BenchmarkHighlightPrompt -benchtime=1x -count=1
```

真实 matcher 使用全部七项 preset（ASCII whole-word），加三项 custom：忽略大小写关键字“性能”“message”、正则 `"id":[0-9]+`。共 161 条编译指令。输入约16KiB，约100字节中文JSON普通文本一行；测完整 transform（含ANSI识别、逐行匹配、优先级marks、SGR输出），编译置于计时外。每次重用h.out，结果必须未降级。无匹配样本为重复普通文本。

提示符以io.Pipe传入无换行的 `prompt> `，每档200次，统计从发起写入到首次stream Read返回；包括goroutine调度及20ms拼片期限，不包括SSH或UI渲染。命令单独执行，不给默认回归增加4秒等待；ns/op表示整批200次。off低于计时分辨率的样本以0微秒输出，不能理解为真实零耗时。

## 原始输出

```
goos: windows
goarch: amd64
pkg: sshub/internal/connection
cpu: Intel(R) Core(TM) Ultra X7 358H
BenchmarkHighlightMatcher/representative-16         	     652	   1648527 ns/op	   9.92 MB/s	  130553 B/op	    4524 allocs/op
BenchmarkHighlightMatcher/no-match-16               	     744	   1594854 ns/op	  10.27 MB/s	   62245 B/op	    1080 allocs/op
PASS
ok  	sshub/internal/connection	2.649s
goos: windows
goarch: amd64
pkg: sshub/internal/connection
cpu: Intel(R) Core(TM) Ultra X7 358H
BenchmarkHighlightPrompt/off-16         	       1	   4504900 ns/op	       527.0 max-us	         0 p50-us	         0 p95-us
BenchmarkHighlightPrompt/on-16          	       1	4092379000 ns/op	     25931 max-us	     20402 p50-us	     20775 p95-us
PASS
ok  	sshub/internal/connection	4.136s
```
