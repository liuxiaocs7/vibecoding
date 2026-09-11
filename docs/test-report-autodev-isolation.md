# Auto-Dev 隔离执行 / 可插拔执行器 / 真实评审 — 测试报告

**测试目标**：`docs/autodev-isolation-executor-review.md`（标注「已落地」的开发设计文档）  
**测试日期**：2026-08-29（含当日自动化缺口补测）  
**测试范围**：`go test` / `go vet` / server 构建 + 实现符合性核对  
**测试环境**：darwin 23.6.0，Go workspace，`CGO_ENABLED=0`（modernc.org/sqlite 无需 CGO）

---

## 1. 结论

| 维度 | 结果 |
|------|------|
| 实现符合设计文档 | ✅ 七个阶段组件均存在（后续重构拆分文件，功能不变） |
| `go test ./...` | ✅ 全部通过，无 skip |
| `go vet ./...` | ✅ 干净 |
| server 形态构建 | ✅ `go build -tags server ./cmd/vibecoding` |
| executor 覆盖率 ≥70% | ✅ **82%+** |
| 文档验收 6 项 | ✅ **5/6 已有自动化等价覆盖**；仅真实 Claude/Cursor 登录 CLI 仍需人工 |

---

## 2. 实现符合性核对

| 设计文档要求 | 代码落点 | 状态 |
|------|------|------|
| `AddWorktree` / `RemoveWorktree` / `MergeBranchAt` | `internal/gitx/`（`gitx.go` + `worktree.go` + `diff.go`） | ✅ |
| `MergeBranchAt` 三路径 | `diff.go`（脏默认分支拒绝；非 base 用临时 worktree） | ✅ |
| Checkout 保留、主路径走 worktree | `gitx.go` + `autodev` worker | ✅ |
| `internal/executor/` | `executor.go` `llm.go` `agent.go` `prompt.go` `probe.go` `presets.go` | ✅ |
| `Executor`：`Name()` / `Run(ctx, CodingRequest, Emit)` | `executor.go` | ✅ |
| `ExecutorConfig` + Normalize/Validate | `model.go` | ✅ |
| `QualityGate` / `WorktreeRef` / PRInfo 扩展字段 | `model.go` | ✅ |
| settings / list executors / issue diff / open-editor / approve-merge | `internal/api` 已注册 | ✅ |
| `Runner.WorktreeRoot` + `NewExecutor` 注入 | `worker.go`；另增 `RunSync` 供同步/测试 | ✅ |
| preset（claude/cursor/codex/custom） | `presets.go` | ✅ |

文件拆分属组织优化，不构成功能偏差。

---

## 3. 自动化测试结果

### 3.1 `go test ./...` — 全绿

含测试的包全部通过，无 `t.Skip`。

### 3.2 覆盖率口径

文档底线是「新代码场景有命名测试」，不是整包立刻 80%。executor 包级 **≥70%** 硬指标已达标。

### 3.3 关键 / 补强用例 → 文档场景

| 场景 | 测试 |
|------|------|
| worktree 隔离、主仓 dirty/HEAD 不变 | `TestAddWorktreeLeavesMainDirtyAndHEAD` 等；`TestRunPreservesMainDirtyAndHEAD`（断言路径 `{WorktreeRoot}/{issueID}/{repoID}`） |
| MergeBranchAt 三路径 | `TestMergeBranchAtCleanOnBase` / `TestMergeBranchAtDirtyOnBaseRefuses` / `TestMergeBranchAtFromOtherBranch` |
| executor fake CLI（不打真 claude） | `TestAgentExecutorFakeCommand` 等 |
| 无 CLI + LLM 仍可用、preset 置灰 | **`TestProbeLLMAvailableWhenCLIsMissing`**；API **`TestListExecutorsMarksLLMWhenKeyConfigured`** |
| heal + repair 日志 + 失败保留 worktree | `TestRunHealRounds`（断言含 `repair round`）；`TestRunKeepsWorktreeOnFailure` |
| 真实 diff（非占位 98/14）+ Approve 成功删 worktree | **`TestAutoDevPipelineRealDiffAndApprove`**；**`TestApproveMergeSuccessRemovesWorktrees`** |
| 脏默认分支 Approve 拒绝 | `TestIssueDiffAndApproveDirty` |

---

## 4. 本机 CLI 探测（旁证，非 CI）

| CLI | 本机 |
|-----|------|
| `claude` | ✅ |
| `cursor-agent` / `agent` | ✅ |
| `codex` | ❌ |

无 CLI 行为由可注入 `LookPath` 的 probe 单测覆盖，不依赖本机 PATH。

---

## 5. 验收项覆盖状态（原「未覆盖」清单）

| # | 验收项 | 状态 | 落点 |
|---|--------|------|------|
| 1 | 非默认分支 + 脏主仓 Start；worktree 出现；主仓不变 | ✅ 自动化 | `TestRunPreservesMainDirtyAndHEAD`；pipeline 同构前置 |
| 2 | 默认 LLM 全流程 → coding/testing → Review 真 diff（≠98/14） | ✅ fake executor | `TestAutoDevPipelineRealDiffAndApprove` |
| 3 | 故意测试失败 → repair 日志 + worktree 保留 | ✅ | `TestRunHealRounds` + `TestRunKeepsWorktreeOnFailure` |
| 4 | 真实 Claude/Cursor 登录 CLI + SSE | ❌ 仍需人工 | 仅有 fake CLI；不代替登录态实跑 |
| 5 | Approve 成功合并并删 worktree；脏默认分支拒绝 | ✅ | success：`TestApproveMergeSuccessRemovesWorktrees` + pipeline；refuse：`TestIssueDiffAndApproveDirty` |
| 6 | 未装 CLI 时 LLM 可用、preset 置灰 | ✅ | `TestProbeLLMAvailableWhenCLIsMissing` 等 |

**仍需人工**：仅第 4 项。建议本地启动 App 后切换 Claude/Cursor preset 跑一条真实 issue。

---

## 6. 总评

设计文档功能已落地且自动化全绿；原先报告第 5 节 6 项手动缺口中，**5 项已用集成/单测等价覆盖**。无法在非交互环境替代的是真实 CLI 登录执行。executor 覆盖率超过 70% 硬指标。
