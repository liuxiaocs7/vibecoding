export type Language = 'en' | 'zh';

export type ThemeStyle = 'glass' | 'slate' | 'oled' | 'oat' | 'light';

export const TRANSLATIONS = {
  en: {
    // Top Bar & Header
    appTitle: 'Vibecoding',
    subtitle: 'Auto-Dev Engine',
    appSubtitle: 'AI Autonomous Dev Engine & Kanban',
    workspace: 'Workspace',
    workspaceProjects: 'Workspace Projects',
    projectSettings: 'Project Settings',
    newIssue: 'New Issue',
    languageName: 'English',
    themeLabel: 'Theme',
    switchLanguageHint: 'Switch language (English / 中文)',
    switchThemeHint: 'Choose visual theme style',
    themeSelection: 'Theme Style Selection',
    
    // Theme Names
    themeGlass: 'Cyber Glass',
    themeSlate: 'Obsidian Slate',
    themeLight: 'Clean Light',
    themeOled: 'Midnight OLED',
    themeOat: 'Warm Oats',

    // Sidebar
    projects: 'Projects',
    newProject: 'New Project',
    gitRepos: 'Git Repositories',
    manage: 'Manage',
    branchPrefixes: 'Branch Prefixes',
    systemSettings: 'System Settings',
    globalLLMConfig: 'Global LLM Model Config',
    llmStatus: 'LLM STATUS',
    customOpenAI: 'Custom OpenAPI',
    serverKeyReady: 'Server Key Injected',

    // Kanban Columns
    colRequirements: 'Requirements Pool',
    colBacklog: 'Backlog / Scheduled',
    colInProgress: 'AI Auto-Dev',
    colInReview: 'PR Review & QA',
    colCompleted: 'Completed & Merged',

    // Priority
    priorityLow: 'Low',
    priorityMedium: 'Medium',
    priorityHigh: 'High',
    priorityUrgent: 'Urgent P0',

    // Card Actions & Badges
    devSpecLabel: 'Dev Spec',
    specGenerated: 'Spec Generated',
    pendingSpec: 'Pending Spec',
    scheduleToBacklog: 'Schedule to Backlog',
    startAutoDev: 'Start Auto-Dev',
    reviewPR: 'Review PR Code',
    viewSpecDiff: 'View Spec & Diff',
    approvedMerged: 'Approved & Merged',
    reviewPending: 'Pending Review',
    autoDevCoding: 'VibeBot Autonomous Coding...',
    mergedBadge: 'MERGED',
    openBadge: 'OPEN',

    // Modal Tabs & Stage Focus
    tabChat: 'AI Chat & Spec',
    tabSpec: 'Dev Spec Document',
    tabConsole: 'Auto-Dev Console',
    tabReview: 'Code Review & Diff',
    
    stageFocusReqTitle: 'Stage Focus: Requirements Ideation & Spec Extraction',
    stageFocusReqDesc: 'Collaborate with AI Copilot to clarify architecture, then extract structured Dev Spec.',
    extractSpecBtn: 'Extract Dev Spec',

    stageFocusBacklogTitle: 'Stage Focus: Confirm Dev Spec & Schedule Execution',
    stageFocusBacklogDesc: 'Target branch prefix verified. Review implementation steps before launching AI Auto-Dev.',

    stageFocusProgressTitle: 'Stage Focus: AI Agent Autonomous Development',
    stageFocusProgressDesc: 'VibeBot is modifying code files, running TypeScript typechecks, and running tests.',
    viewLogsBtn: 'Live Terminal Logs',

    stageFocusReviewTitle: 'Stage Focus: Code PR Review & Quality Gate',
    stageFocusReviewDesc: 'Unit tests and static analysis are 100% green. Inspect code diffs before merging.',
    approveMergeBtn: 'Approve & Merge',

    stageFocusCompletedTitle: 'Stage Focus: Delivered & Archived',
    stageFocusCompletedDesc: 'Branch successfully merged into main. Complete Spec & quality report are archived.',

    // Review Tab Details
    codeReviewReport: 'Code Quality & Review Report',
    unitTestPass: '100% Tests Passed',
    tsTypecheckPass: '0 Type Errors',
    eslintPass: '0 ESLint Warnings',
    linesChanged: 'Lines Changed',
    reviewerComments: 'Reviewer Comments:',
    requestRework: 'Request Rework',
    fileDiffHeader: 'File Code Diff',
    originalCode: 'Original Code',
    modifiedCode: 'AI Modified / Added',
    reworkPrompt: 'Describe required changes or bug fixes for the AI agent:',

    // Common Buttons
    cancel: 'Cancel',
    save: 'Save',
    close: 'Close',
    confirm: 'Confirm',

    // App shell / toasts
    loadingApp: 'Loading Vibecoding…',
    backendUnavailable: 'Backend unavailable',
    startBackendHint: 'Start the Go server on :8090 (`make backend && ./vibecoding`)',
    loadBackendFailed: 'Failed to load from backend',
    llmSettingsSaved: 'LLM settings saved',
    projectDeleted: 'Project deleted',
    noProjectsYet: 'No projects yet. Create one to get started.',
    deleteCurrentProject: 'Delete current project',
    llmNotConfigured: 'LLM NOT CONFIGURED',
    llmReady: 'Ready',
    openSettingsAddKey: 'Open settings to add API key',
    workspaceFallback: 'Vibecoding Workspace',
    workspaceHint: 'Create a project and link a local git repository to begin.',
    createProjectToStart: 'Create a project to start vibecoding.',
    autoDevAlreadyRunning: 'Auto-Dev already running for this issue',
    autoDevNeedsSpec: 'Dev Spec required before Auto-Dev',
    autoDevNeedsRepo: 'Associate at least one repository',
    autoDevFailed: 'Auto-Dev failed',
    autoDevStartFailed: 'Failed to start Auto-Dev',
    autoDevCancelRequested: 'Auto-Dev cancel requested',
    backlogNeedsSpec: 'Markdown Dev Spec required before backlog',
    backlogNeedsRepo: 'Associate a repository before backlog',
    reviewNeedsAutoDev: 'Complete Auto-Dev before moving to review',
    openaiPrefix: 'OPENAPI',
    customModel: 'CUSTOM',

    // Issue detail
    acceptSpecBacklog: 'Accept Spec → Backlog',
    deleteIssueConfirm: 'Delete this issue?',
    deleteIssueTitle: 'Delete issue',
    requestCancelled: 'Request cancelled',
    llmError: 'LLM error',
    mergeFailed: 'Merge failed',
    scoreMerged: 'Score: 100 / 100 (Merged)',
    scorePassed: 'Score: 100 / 100 (PASSED)',
    unitTestPassed: '100% Passed',
    zeroTypeErrors: '0 Type Errors',
    zeroWarnings: '0 Warnings',
    processDone: 'done',
    processError: 'error',
    processStreaming: 'streaming',
    autoExecLogHeader: '=== Vibecoding Auto Execution Log ===',
    vibeBotConsoleTitle: 'VibeBot Autonomous Code Agent Logs',

    // Project / settings
    validatePath: 'Validate path',
    pathRequired: 'Path required',
    invalidRepository: 'Invalid repository',
    validationFailed: 'Validation failed',
    validatedRepo: 'Validated',
    filesLabel: 'files',
    branchLabel: 'branch',
    baseUrlLabel: 'Base URL',
    apiKeyLabel: 'API Key',
    modelNameLabel: 'Model Name',
    keyConfiguredKeep: 'Configured ({hint}) — leave blank to keep',
    keyPlaceholderServer: 'sk-... (stored server-side only)',
    keyStoredHint: 'Key is stored on the local Go server ({hint}). Paste a new key only to replace it.',
    repoTooltip: 'Repo',
    branchTooltip: 'Branch',
    defaultAssignee: 'AI Auto-Dev Agent',
    issueInitLog: '[Init] Issue initialized in {status} status',
  },

  zh: {
    // Top Bar & Header
    appTitle: 'Vibecoding',
    subtitle: 'AI 自治开发引擎',
    appSubtitle: 'AI 自治开发看板与代码全流程',
    workspace: '工作空间',
    workspaceProjects: '工作区项目工程',
    projectSettings: '项目工程设置',
    newIssue: '新建 Issue 需求',
    languageName: '简体中文',
    themeLabel: '主题样式',
    switchLanguageHint: '切换语言 (English / 简体中文)',
    switchThemeHint: '选择 UI 主题视觉风格',
    themeSelection: '展示样式选择',

    // Theme Names
    themeGlass: '赛博玻璃 (Dark Glass)',
    themeSlate: '黑曜石 (Obsidian Slate)',
    themeLight: '明亮极简 (Clean Light)',
    themeOled: '纯黑极客 (Midnight OLED)',
    themeOat: '暖阳燕麦 (Warm Oats)',

    // Sidebar
    projects: '关联项目',
    newProject: '新建项目',
    gitRepos: 'Git 关联代码工程',
    manage: '管理',
    branchPrefixes: '分支前缀规范',
    systemSettings: '系统设置',
    globalLLMConfig: '全局 LLM 大模型配置',
    llmStatus: '大模型状态',
    customOpenAI: '自定义 OpenAPI',
    serverKeyReady: '服务端 Key 就绪',

    // Kanban Columns
    colRequirements: '需求池 / 待探讨',
    colBacklog: '待排期 / Ready',
    colInProgress: '自治编码中',
    colInReview: '待代码评审 (PR)',
    colCompleted: '已完成与合并',

    // Priority
    priorityLow: '低优先级',
    priorityMedium: '中优先级',
    priorityHigh: '高优先级',
    priorityUrgent: '紧急 P0',

    // Card Actions & Badges
    devSpecLabel: '待开发文档 Spec',
    specGenerated: '已生成 Spec',
    pendingSpec: '待生成 Spec',
    scheduleToBacklog: '排期待执行',
    startAutoDev: '启动自治开发',
    reviewPR: '评审 PR 代码',
    viewSpecDiff: '查看 Spec & Diff',
    approvedMerged: '代码评审通过 & 已合并',
    reviewPending: '待代码评审',
    autoDevCoding: 'VibeBot 自治编码中...',
    mergedBadge: '已合并',
    openBadge: '待评审',

    // Modal Tabs & Stage Focus
    tabChat: 'AI 对话与 Spec 提炼',
    tabSpec: '待开发文档 (Dev Spec)',
    tabConsole: '自动开发控制台',
    tabReview: '代码评审与 Git Diff',

    stageFocusReqTitle: '阶段侧重点: 需求探讨与 Dev Spec 生成',
    stageFocusReqDesc: '通过 AI Copilot 进行需求拆解与架构讨论，确认后提炼规范。',
    extractSpecBtn: '提炼 Dev Spec',

    stageFocusBacklogTitle: '阶段侧重点: 确认 Dev Spec & 排期准备',
    stageFocusBacklogDesc: '已匹配目标分支规则，确认开发步骤与涉及代码后即可启动自治编码。',

    stageFocusProgressTitle: '阶段侧重点: AI 智能体自治编码与监控',
    stageFocusProgressDesc: 'VibeBot 正自动修改代码文件并执行 TypeScript 类型检查与单元测试。',
    viewLogsBtn: '实时 Shell Logs',

    stageFocusReviewTitle: '阶段侧重点: 代码 PR 审查与质量门禁',
    stageFocusReviewDesc: '单元测试与静态分析 100% 绿灯，请仔细审核变动文件 Code Diff。',
    approveMergeBtn: '同意合并 (Approve & Merge)',

    stageFocusCompletedTitle: '阶段侧重点: 已完成交付与全额归档',
    stageFocusCompletedDesc: '需求对应分支已成功合并至 main 主干，完整 Spec 与质量报告已存储。',

    // Review Tab Details
    codeReviewReport: '代码自动化评审与质量门禁结果',
    unitTestPass: '100% 单元测试通过',
    tsTypecheckPass: '0 类型报错',
    eslintPass: '0 静态警告',
    linesChanged: '修改行数与体积',
    reviewerComments: '评审结论与点评意见:',
    requestRework: '打回重新迭代',
    fileDiffHeader: '变更文件 Diff 详情对比',
    originalCode: '原始代码 (Original)',
    modifiedCode: 'AI 修改/新生成的代码',
    reworkPrompt: '请输入给 AI Agent 的修改要求或 Bug 反馈:',

    // Common Buttons
    cancel: '取消',
    save: '保存',
    close: '关闭',
    confirm: '确认',

    // App shell / toasts
    loadingApp: '正在加载 Vibecoding…',
    backendUnavailable: '后端不可用',
    startBackendHint: '请在 :8090 启动 Go 服务（`make backend && ./vibecoding`）',
    loadBackendFailed: '无法从后端加载数据',
    llmSettingsSaved: 'LLM 设置已保存',
    projectDeleted: '项目已删除',
    noProjectsYet: '暂无项目，请先创建一个开始使用',
    deleteCurrentProject: '删除当前项目',
    llmNotConfigured: '未配置 LLM',
    llmReady: '已就绪',
    openSettingsAddKey: '请打开设置添加 API 密钥',
    workspaceFallback: 'Vibecoding 工作区',
    workspaceHint: '创建项目并关联本地 Git 仓库即可开始',
    createProjectToStart: '请先创建项目以开始 Vibecoding',
    autoDevAlreadyRunning: '该 Issue 的自治开发已在运行',
    autoDevNeedsSpec: '启动自治开发前需先有待开发文档',
    autoDevNeedsRepo: '请至少关联一个仓库',
    autoDevFailed: '自治开发失败',
    autoDevStartFailed: '启动自治开发失败',
    autoDevCancelRequested: '已请求取消自治开发',
    backlogNeedsSpec: '移入待执行前需先有 Markdown 开发文档',
    backlogNeedsRepo: '移入待执行前请先关联仓库',
    reviewNeedsAutoDev: '请先完成自治开发再移至评审',
    openaiPrefix: 'OpenAPI',
    customModel: '自定义',

    // Issue detail
    acceptSpecBacklog: '接受规范 → 待执行',
    deleteIssueConfirm: '确定删除该 Issue？',
    deleteIssueTitle: '删除 Issue',
    requestCancelled: '请求已取消',
    llmError: 'LLM 调用出错',
    mergeFailed: '合并失败',
    scoreMerged: '评分：100 / 100（已合并）',
    scorePassed: '评分：100 / 100（通过）',
    unitTestPassed: '100% 通过',
    zeroTypeErrors: '0 类型错误',
    zeroWarnings: '0 警告',
    processDone: '完成',
    processError: '错误',
    processStreaming: '流式输出中',
    autoExecLogHeader: '=== Vibecoding 自动执行日志 ===',
    vibeBotConsoleTitle: 'VibeBot 自治编码 Agent 执行日志',

    // Project / settings
    validatePath: '校验路径',
    pathRequired: '请填写路径',
    invalidRepository: '无效的仓库',
    validationFailed: '校验失败',
    validatedRepo: '校验通过',
    filesLabel: '个文件',
    branchLabel: '分支',
    baseUrlLabel: '接口基址 (Base URL)',
    apiKeyLabel: 'API 密钥',
    modelNameLabel: '模型名称',
    keyConfiguredKeep: '已配置（{hint}）— 留空则保持不变',
    keyPlaceholderServer: 'sk-...（仅保存在服务端）',
    keyStoredHint: '密钥已保存在本地 Go 服务（{hint}）。仅在需要更换时粘贴新密钥。',
    repoTooltip: '仓库',
    branchTooltip: '分支',
    defaultAssignee: 'AI 自治开发 Agent',
    issueInitLog: '[初始化] Issue 已创建，当前状态：{status}',
  },
};

export function getTranslation(lang?: string | Language) {
  if (lang === 'zh') return TRANSLATIONS.zh;
  return TRANSLATIONS.en;
}
