export type Priority = 'low' | 'medium' | 'high' | 'urgent';

export type IssueStatus = 'requirements' | 'backlog' | 'in_progress' | 'in_review' | 'completed';

export interface GitRepo {
  id: string;
  name: string;
  path: string; // 本地 Git 仓库绝对/相对磁盘路径 (例如 /Users/dev/workspace/order-service)
  defaultBranch: string;
  language: string;
  description?: string;
  filesCount?: number;
  url?: string; // 可选的远程备份 URL
}

export interface ModelConfig {
  useCustomOpenAI: boolean;
  openAIBaseUrl: string; // e.g. https://api.openai.com/v1
  openAIApiKey: string;
  openAIModel: string; // e.g. gpt-4o, gpt-4o-mini, deepseek-chat, deepseek-r1
  temperature: number;
  keyConfigured?: boolean;
  keyHint?: string;
}

export interface BranchPrefixConfig {
  featurePrefix: string;  // 特性开发分支前缀 (如 feature/ 或 feat/)
  bugfixPrefix: string;   // 缺陷修复分支前缀 (如 fix/ 或 bugfix/)
  hotfixPrefix: string;   // 紧急修复分支前缀 (如 hotfix/)
  refactorPrefix: string; // 代码重构分支前缀 (如 refactor/)
  autoDevPrefix: string;  // AI自治改码分支前缀 (如 ai-dev/ 或 vibe/)
  releasePrefix: string;  // 版本发布分支前缀 (如 release/)
}

export interface Project {
  id: string;
  name: string;
  description: string;
  gitRepos: GitRepo[];
  branchPrefixConfig?: BranchPrefixConfig; // 工程级别 Git 分支前缀规范配置
  customModelConfig?: ModelConfig;
  useCustomModelConfig: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface ChatMessage {
  id: string;
  sender: 'user' | 'ai' | 'system';
  text: string;
  timestamp: string;
}

export interface SpecFileChange {
  filePath: string;
  repoName: string;
  action: 'create' | 'modify' | 'delete';
  summary: string;
  originalCode?: string;
  modifiedCode?: string;
}

export interface DevSpec {
  title: string;
  summary: string;
  architectureDesign: string;
  fileChanges: SpecFileChange[];
  implementationSteps: string[];
  testCases: string[];
  rawMarkdown: string;
  updatedAt: string;
}

export interface AutoDevLog {
  id: string;
  timestamp: string;
  phase: 'analyzing' | 'branching' | 'coding' | 'testing' | 'linting' | 'committing' | 'completed' | 'failed';
  message: string;
  details?: string;
}

export interface PRInfo {
  id: string;
  branchName: string;
  title: string;
  description: string;
  author: string;
  createdAt: string;
  status: 'open' | 'merged' | 'rejected';
  diffStats: {
    additions: number;
    deletions: number;
    filesChanged: number;
  };
}

export interface Issue {
  id: string;
  projectId: string;
  title: string;
  description: string;
  priority: Priority;
  status: IssueStatus;
  associatedRepoIds: string[];
  assignee: string;
  chatMessages: ChatMessage[];
  devSpec?: DevSpec;
  autoDevLogs: AutoDevLog[];
  autoDevProgress: number; // 0 - 100
  prInfo?: PRInfo;
  reviewFeedback?: string;
  createdAt: string;
  updatedAt: string;
}
