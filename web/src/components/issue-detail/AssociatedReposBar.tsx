import React from 'react';
import { Issue, GitRepo } from '../../types';
import { Language, ThemeStyle } from '../../lib/i18n';
import { THEME_CONFIGS } from '../../lib/theme';
import { GitBranch, Pencil } from 'lucide-react';

interface AssociatedReposBarProps {
  issue: Issue;
  gitRepos: GitRepo[];
  associatedRepos: GitRepo[];
  canEditRepos: boolean;
  editingRepos: boolean;
  setEditingRepos: React.Dispatch<React.SetStateAction<boolean>>;
  toggleAssociatedRepo: (repoId: string) => void;
  lang: Language;
  themeStyle: ThemeStyle;
}

export const AssociatedReposBar: React.FC<AssociatedReposBarProps> = ({
  issue,
  gitRepos,
  associatedRepos,
  canEditRepos,
  editingRepos,
  setEditingRepos,
  toggleAssociatedRepo,
  lang,
  themeStyle,
}) => {
  const themeConfig = THEME_CONFIGS[themeStyle] || THEME_CONFIGS.light;

  return (
    <div className={`px-5 py-2.5 border-b ${themeConfig.subtleBorder} ${themeConfig.modalHeaderBg}`}>
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0 flex-1">
          <div className={`flex items-center gap-1.5 text-[10px] uppercase tracking-wider font-bold mb-1.5 ${themeConfig.textMuted}`}>
            <GitBranch className="w-3 h-3" />
            {lang === 'zh' ? '关联仓库' : 'Associated Repos'}
          </div>
          {editingRepos ? (
            gitRepos.length === 0 ? (
              <p className="text-xs text-rose-400">
                {lang === 'zh' ? '项目尚未配置仓库，请先在项目设置中添加。' : 'No repos in project. Add them in project settings first.'}
              </p>
            ) : (
              <div className="flex flex-wrap gap-2">
                {gitRepos.map((repo) => {
                  const checked = issue.associatedRepoIds.includes(repo.id);
                  return (
                    <button
                      key={repo.id}
                      type="button"
                      disabled={!canEditRepos}
                      onClick={() => toggleAssociatedRepo(repo.id)}
                      className={`px-2.5 py-1.5 rounded-lg border text-left text-[11px] transition-colors max-w-full ${
                        checked
                          ? 'border-indigo-500/50 bg-indigo-500/15 text-indigo-800 dark:text-indigo-200'
                          : `${themeConfig.inputBg} ${themeConfig.inputBorder} ${themeConfig.textSecondary}`
                      } ${!canEditRepos ? 'opacity-60 cursor-not-allowed' : ''}`}
                      title={repo.path}
                    >
                      <span className="font-semibold">{repo.name}</span>
                      <span className={`block font-mono truncate max-w-[220px] ${themeConfig.textMuted}`}>
                        {repo.path || '—'}
                      </span>
                    </button>
                  );
                })}
              </div>
            )
          ) : (
            <p
              className={`text-xs truncate select-text ${themeConfig.textSecondary}`}
              title={associatedRepos.map((r) => `${r.name} (${r.path || ''})`).join(', ')}
            >
              {associatedRepos.length > 0
                ? associatedRepos.map((r) => `${r.name}`).join(', ')
                : lang === 'zh'
                ? '未关联仓库'
                : 'No repos linked'}
            </p>
          )}
        </div>
        {canEditRepos && (
          <button
            type="button"
            onClick={() => setEditingRepos((v) => !v)}
            className={`shrink-0 px-2.5 py-1 rounded-lg text-[11px] font-semibold border flex items-center gap-1 ${themeConfig.btnSecondary} ${themeConfig.btnSecondaryText}`}
          >
            <Pencil className="w-3 h-3" />
            {editingRepos
              ? lang === 'zh'
                ? '完成'
                : 'Done'
              : lang === 'zh'
              ? '修改'
              : 'Edit'}
          </button>
        )}
      </div>
    </div>
  );
};
