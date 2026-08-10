import React, { useState } from 'react';
import { Issue, GitRepo, Priority, IssueStatus } from '../types';
import { Language, ThemeStyle, getTranslation } from '../lib/i18n';
import { THEME_CONFIGS } from '../lib/theme';
import { X, PlusCircle, GitBranch, AlertCircle, Sparkles, Check, Repeat } from 'lucide-react';

interface CreateIssueModalProps {
  isOpen: boolean;
  onClose: () => void;
  projectId: string;
  gitRepos: GitRepo[];
  onCreate: (issue: Partial<Issue>) => void;
  lang?: Language;
  themeStyle?: ThemeStyle;
}

export const CreateIssueModal: React.FC<CreateIssueModalProps> = ({
  isOpen,
  onClose,
  projectId,
  gitRepos,
  onCreate,
  lang = 'en',
  themeStyle = 'glass',
}) => {
  const t = getTranslation(lang);
  const themeConfig = THEME_CONFIGS[themeStyle] || THEME_CONFIGS.glass;
  const isLight = themeConfig.isLight;

  const [title, setTitle] = useState('');
  const [description, setDescription] = useState('');
  const [priority, setPriority] = useState<Priority>('medium');
  const [status, setStatus] = useState<IssueStatus>('requirements');
  const [selectedRepoIds, setSelectedRepoIds] = useState<string[]>(
    gitRepos.map((r) => r.id)
  );
  const [assignee, setAssignee] = useState(
    lang === 'zh' ? 'AI 自治开发 Agent' : 'AI Auto-Dev Agent'
  );
  const [keepOpenAfterCreate, setKeepOpenAfterCreate] = useState(false);
  const [createdCount, setCreatedCount] = useState(0);

  if (!isOpen) return null;

  const toggleRepoSelection = (repoId: string) => {
    if (selectedRepoIds.includes(repoId)) {
      if (selectedRepoIds.length <= 1) {
        alert(lang === 'zh' ? '请至少勾选一个关联的 Git 工程' : 'Please select at least one associated Git repository');
        return;
      }
      setSelectedRepoIds(selectedRepoIds.filter((id) => id !== repoId));
    } else {
      setSelectedRepoIds([...selectedRepoIds, repoId]);
    }
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!title.trim()) {
      alert(lang === 'zh' ? '请输入 Issue 标题' : 'Please enter an issue title');
      return;
    }
    if (selectedRepoIds.length === 0) {
      alert(lang === 'zh' ? '请选择至少一个关联的 Git 仓库' : 'Please select at least one Git repo');
      return;
    }

    const newIssue: Partial<Issue> = {
      id: `ISSUE-${Date.now().toString().slice(-4)}`,
      projectId,
      title: title.trim(),
      description: description.trim() || (lang === 'zh' ? '暂无详细描述信息' : 'No description provided.'),
      status,
      priority,
      assignee,
      associatedRepoIds: selectedRepoIds,
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
      autoDevProgress: status === 'in_progress' ? 15 : 0,
      chatMessages: [
        {
          id: `msg-${Date.now()}`,
          sender: 'system',
          text: lang === 'zh' 
            ? `系统提示: Issue 已成功创建，关联仓库: ${selectedRepoIds.map(id => gitRepos.find(r => r.id === id)?.name).join(', ')}。可在此对话框输入需求细节，并随时按 [提炼 Dev Spec] 生成规范文档。`
            : `System: Issue created and linked to repo(s): ${selectedRepoIds.map(id => gitRepos.find(r => r.id === id)?.name).join(', ')}. Detail your requirement here and click [Extract Dev Spec] anytime.`,
          timestamp: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
        },
      ],
      autoDevLogs: [
        {
          id: `log-${Date.now()}`,
          timestamp: new Date().toLocaleTimeString(),
          phase: 'analyzing',
          message: t.issueInitLog.replace('{status}', status),
        },
      ],
    };

    onCreate(newIssue);
    setCreatedCount((prev) => prev + 1);

    if (keepOpenAfterCreate) {
      setTitle('');
      setDescription('');
    } else {
      onClose();
    }
  };

  const statusOptions: { id: IssueStatus; label: string; badgeStyle: string }[] = [
    { id: 'requirements', label: '需求列表 (Pool)', badgeStyle: isLight ? 'bg-cyan-100 text-cyan-900 border-cyan-300' : 'bg-cyan-500/20 text-cyan-300 border-cyan-500/30' },
  ];

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-md p-4">
      <div className={`w-full max-w-xl border rounded-2xl shadow-2xl overflow-hidden flex flex-col ${themeConfig.modalBg}`}>
        {/* Modal Header */}
        <div className={`p-6 border-b flex items-center justify-between ${themeConfig.subtleBorder} ${themeConfig.modalHeaderBg}`}>
          <div className="flex items-center gap-3">
            <div className="p-2.5 rounded-xl bg-indigo-500/20 border border-indigo-500/30 text-indigo-500">
              <PlusCircle className="w-6 h-6" />
            </div>
            <div>
              <h2 className={`text-xl font-bold flex items-center gap-2 ${themeConfig.textPrimary}`}>
                新建 Issue 需求
                {createdCount > 0 && (
                  <span className="text-xs px-2 py-0.5 rounded-full bg-emerald-500/20 text-emerald-600 dark:text-emerald-300 border border-emerald-500/30 font-normal">
                    已连续创建 {createdCount} 个需求
                  </span>
                )}
              </h2>
              <p className={`text-xs mt-0.5 ${themeConfig.textSecondary}`}>
                支持批量/连续创建需求，并灵活选择需求对应的初始状态
              </p>
            </div>
          </div>
          <button
            onClick={onClose}
            className={`p-2 rounded-lg transition-colors ${themeConfig.textSecondary} hover:${themeConfig.textPrimary} hover:bg-black/5 dark:hover:bg-white/10`}
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Modal Body */}
        <form onSubmit={handleSubmit} className="p-6 space-y-5 text-sm">
          {/* Title */}
          <div>
            <label className={`block text-xs font-semibold mb-1.5 ${themeConfig.textPrimary}`}>
              Issue 标题 <span className="text-rose-500">*</span>
            </label>
            <input
              type="text"
              required
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder="例如: 增加 Redis 接口请求防重锁与全局异常处理"
              className={`w-full px-3.5 py-2.5 border rounded-xl focus:outline-none transition-colors ${themeConfig.inputBg} ${themeConfig.inputText} ${themeConfig.inputBorder}`}
            />
          </div>

          {/* Description */}
          <div>
            <label className={`block text-xs font-semibold mb-1.5 ${themeConfig.textPrimary}`}>
              需求描述 / 业务背景
            </label>
            <textarea
              rows={3}
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="详细描述需要开发或修改的功能点、输入输出规格以及报错处理逻辑..."
              className={`w-full px-3.5 py-2.5 border rounded-xl focus:outline-none transition-colors text-xs leading-relaxed ${themeConfig.inputBg} ${themeConfig.inputText} ${themeConfig.inputBorder}`}
            />
          </div>

          {/* Initial Status Selection */}
          <div>
            <label className={`block text-xs font-semibold mb-1.5 ${themeConfig.textPrimary}`}>
              需求初始状态 (可自由指定)
            </label>
            <div className="grid grid-cols-2 sm:grid-cols-4 gap-2">
              {statusOptions.map((opt) => {
                const isSelected = status === opt.id;
                return (
                  <button
                    type="button"
                    key={opt.id}
                    onClick={() => setStatus(opt.id)}
                    className={`px-2.5 py-2 rounded-xl text-xs font-medium border flex items-center justify-between transition-all ${
                      isSelected
                        ? `${opt.badgeStyle} ring-1 ring-indigo-500/50 shadow-md font-bold`
                        : `${themeConfig.inputBg} ${themeConfig.inputBorder} ${themeConfig.textSecondary}`
                    }`}
                  >
                    <span className="truncate">{opt.label.split(' ')[0]}</span>
                    {isSelected && <Check className="w-3.5 h-3.5 shrink-0" />}
                  </button>
                );
              })}
            </div>
          </div>

          {/* Priority & Assignee */}
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className={`block text-xs font-semibold mb-1.5 ${themeConfig.textPrimary}`}>优先级</label>
              <select
                value={priority}
                onChange={(e) => setPriority(e.target.value as Priority)}
                className={`w-full px-3 py-2 border rounded-xl focus:outline-none text-xs ${themeConfig.inputBg} ${themeConfig.inputText} ${themeConfig.inputBorder}`}
              >
                <option value="low" className={isLight ? 'bg-white text-slate-900' : 'bg-slate-900 text-white'}>低 (Low)</option>
                <option value="medium" className={isLight ? 'bg-white text-slate-900' : 'bg-slate-900 text-white'}>中 (Medium)</option>
                <option value="high" className={isLight ? 'bg-white text-slate-900' : 'bg-slate-900 text-white'}>高 (High)</option>
                <option value="urgent" className={isLight ? 'bg-white text-slate-900' : 'bg-slate-900 text-white'}>紧急 (Urgent)</option>
              </select>
            </div>

            <div>
              <label className={`block text-xs font-semibold mb-1.5 ${themeConfig.textPrimary}`}>负责人 / Agent</label>
              <input
                type="text"
                value={assignee}
                onChange={(e) => setAssignee(e.target.value)}
                placeholder={
                  lang === 'zh' ? 'AI 自治开发 Agent / 开发人员姓名' : 'AI Auto-Dev Agent / developer name'
                }
                className={`w-full px-3 py-2 border rounded-xl focus:outline-none text-xs ${themeConfig.inputBg} ${themeConfig.inputText} ${themeConfig.inputBorder}`}
              />
            </div>
          </div>

          {/* Associated Git Repositories */}
          <div>
            <label className={`block text-xs font-semibold mb-2 flex items-center justify-between ${themeConfig.textPrimary}`}>
              <span className="flex items-center gap-1.5">
                <GitBranch className="w-4 h-4 text-indigo-500" />
                关联 Git 代码工程 <span className="text-rose-500">*</span>
              </span>
              <span className={`text-[11px] ${themeConfig.textMuted}`}>已选择 {selectedRepoIds.length} 个</span>
            </label>

            {gitRepos.length === 0 ? (
              <div className="p-3 bg-rose-500/20 border border-rose-500/30 rounded-xl text-xs text-rose-700 dark:text-rose-200 flex items-center gap-2">
                <AlertCircle className="w-4 h-4 text-rose-500" />
                当前项目暂未关联 Git 仓库，请先在项目设置中添加 Git 工程。
              </div>
            ) : (
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-2.5 max-h-36 overflow-y-auto">
                {gitRepos.map((repo) => {
                  const isChecked = selectedRepoIds.includes(repo.id);
                  return (
                    <div
                      key={repo.id}
                      onClick={() => toggleRepoSelection(repo.id)}
                      className={`p-2.5 rounded-xl border cursor-pointer flex items-center justify-between transition-all ${
                        isChecked
                          ? 'border-indigo-500 bg-indigo-500/15 text-indigo-900 dark:text-indigo-200 font-semibold'
                          : `${themeConfig.inputBg} ${themeConfig.inputBorder} ${themeConfig.textSecondary}`
                      }`}
                    >
                      <div className="overflow-hidden pr-2">
                        <div className={`font-semibold text-xs truncate ${themeConfig.textPrimary}`}>
                          {repo.name}
                        </div>
                        <div className={`text-[10px] truncate font-mono mt-0.5 ${themeConfig.textMuted}`} title={repo.path}>
                          📁 {repo.path || repo.url || '未设置本地路径'}
                        </div>
                      </div>
                      <input
                        type="checkbox"
                        checked={isChecked}
                        onChange={() => {}}
                        className="rounded border-slate-300 bg-slate-100 text-indigo-600 focus:ring-indigo-500"
                      />
                    </div>
                  );
                })}
              </div>
            )}
          </div>

          {/* Footer Submit & Continuous creation toggle */}
          <div className={`pt-4 border-t flex items-center justify-between gap-3 ${themeConfig.subtleBorder}`}>
            <label className={`flex items-center gap-2 text-xs cursor-pointer select-none ${themeConfig.textSecondary}`}>
              <input
                type="checkbox"
                checked={keepOpenAfterCreate}
                onChange={(e) => setKeepOpenAfterCreate(e.target.checked)}
                className="rounded border-slate-300 bg-slate-100 text-indigo-600 focus:ring-indigo-500"
              />
              <span className="flex items-center gap-1">
                <Repeat className="w-3.5 h-3.5 text-indigo-500" />
                连续创建模式 (保存后不关闭弹窗)
              </span>
            </label>

            <div className="flex items-center gap-2">
              <button
                type="button"
                onClick={onClose}
                className={`px-4 py-2 rounded-xl text-xs font-medium border transition-colors ${themeConfig.btnSecondary} ${themeConfig.btnSecondaryText}`}
              >
                取消
              </button>
              <button
                type="submit"
                className="px-5 py-2 rounded-xl text-xs font-semibold text-white bg-indigo-600 hover:bg-indigo-500 shadow-md transition-all flex items-center gap-2"
              >
                <Sparkles className="w-4 h-4" />
                创建需求 ({
                  status === 'requirements'
                    ? '存入需求列表'
                    : status === 'backlog'
                    ? '存入待执行'
                    : status === 'in_progress'
                    ? '存入进行中'
                    : status === 'in_review'
                    ? '存入待评审'
                    : '存入已完成'
                })
              </button>
            </div>
          </div>
        </form>
      </div>
    </div>
  );
};

;
