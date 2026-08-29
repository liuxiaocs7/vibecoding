import React, { useState, useEffect, useRef } from 'react';
import { Issue, GitRepo, ModelConfig, BranchPrefixConfig, SubRequirement } from '../../types';
import { Language, getTranslation, ThemeStyle } from '../../lib/i18n';
import { THEME_CONFIGS } from '../../lib/theme';
import { api } from '../../lib/api';
import { hasSubRequirements, specMarkdownForExport, visibleSpec } from '../../lib/subreq';
import { saveTextFile } from '../../lib/savefile';
import { useIssueChat } from './useIssueChat';
import { IssueDetailHeader } from './IssueDetailHeader';
import { AssociatedReposBar } from './AssociatedReposBar';
import { StatusFocusBanner } from './StatusFocusBanner';
import { IssueDetailTabs } from './IssueDetailTabs';
import { IssueChatTab } from './IssueChatTab';
import { IssueSpecTab } from './IssueSpecTab';
import { IssueConsoleTab } from './IssueConsoleTab';
import { IssueReviewTab } from './IssueReviewTab';

interface IssueDetailModalProps {
  isOpen: boolean;
  onClose: () => void;
  issue: Issue;
  gitRepos: GitRepo[];
  modelConfig: ModelConfig;
  branchPrefixConfig?: BranchPrefixConfig;
  onUpdateIssue: (updatedIssue: Issue) => void;
  onStartAutoDev: (issueId: string, subRequirementId?: string) => void;
  onCancelAutoDev?: (issueId: string) => void;
  onDeleteIssue?: (issueId: string) => void;
  projectId?: string;
  lang?: Language;
  themeStyle?: ThemeStyle;
}

export const IssueDetailModal: React.FC<IssueDetailModalProps> = ({
  isOpen,
  onClose,
  issue,
  gitRepos,
  modelConfig,
  branchPrefixConfig,
  onUpdateIssue,
  onStartAutoDev,
  onCancelAutoDev,
  onDeleteIssue,
  projectId,
  lang = 'en' as Language,
  themeStyle = 'glass',
}) => {
  const themeConfig = THEME_CONFIGS[themeStyle] || THEME_CONFIGS.glass;
  const t = getTranslation(lang);

  const [activeTab, setActiveTab] = useState<'chat' | 'spec' | 'console' | 'review'>('chat');
  const [editingSpec, setEditingSpec] = useState(false);

  const [specMarkdown, setSpecMarkdown] = useState(issue.devSpec?.rawMarkdown || '');
  const [reworkFeedback, setReworkFeedback] = useState('');
  const [showReworkBox, setShowReworkBox] = useState(false);
  const [editingRepos, setEditingRepos] = useState(false);
  const [selectedScope, setSelectedScope] = useState<string>('all');
  const [reworkScope, setReworkScope] = useState<string>('all');
  const [exporting, setExporting] = useState(false);
  const [exportHint, setExportHint] = useState('');

  const messagesEndRef = useRef<HTMLDivElement>(null);
  const splitIssue = hasSubRequirements(issue);
  const activeSub: SubRequirement | undefined =
    splitIssue && selectedScope !== 'all'
      ? (issue.subRequirements || []).find((s) => s.id === selectedScope)
      : undefined;
  const visibleMessages = activeSub ? activeSub.chatMessages || [] : issue.chatMessages;
  const currentSpec = visibleSpec(issue, selectedScope);

  const associatedRepos = gitRepos.filter((r) => issue.associatedRepoIds.includes(r.id));
  const canEditRepos = issue.status === 'requirements' || issue.status === 'backlog';

  const {
    inputPrompt,
    setInputPrompt,
    isSending,
    chatError,
    setChatError,
    modelProcess,
    setModelProcess,
    abortRef,
    handleSendMessage,
  } = useIssueChat({
    issue,
    splitIssue,
    selectedScope,
    setSelectedScope,
    setSpecMarkdown,
    onUpdateIssue,
    modelConfig,
    projectId,
    lang,
    associatedRepos,
  });

  // Automatically switch active tab based on Issue status when modal opens
  useEffect(() => {
    if (!isOpen) return;
    setEditingRepos(false);
    setModelProcess({ entries: [], expanded: false });
    setSelectedScope('all');
    setReworkScope('all');
    setExportHint('');
    if (issue.status === 'requirements') {
      setActiveTab('chat');
    } else if (issue.status === 'backlog') {
      setActiveTab('spec');
    } else if (issue.status === 'in_progress') {
      setActiveTab('console');
    } else if (issue.status === 'in_review' || issue.status === 'completed') {
      setActiveTab('review');
    }
  }, [isOpen, issue.id, issue.status, setModelProcess]);

  useEffect(() => {
    const spec = visibleSpec(issue, selectedScope);
    if (spec?.rawMarkdown) {
      setSpecMarkdown(spec.rawMarkdown);
    }
  }, [issue.devSpec, issue.subRequirements, selectedScope]);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [visibleMessages, activeTab]);

  if (!isOpen) return null;

  const toggleAssociatedRepo = (repoId: string) => {
    if (!canEditRepos) return;
    const current = issue.associatedRepoIds || [];
    const next = current.includes(repoId)
      ? current.filter((id) => id !== repoId)
      : [...current, repoId];
    if (next.length === 0) {
      setChatError(lang === 'zh' ? '请至少保留一个关联仓库' : 'Keep at least one associated repository');
      return;
    }
    setChatError('');
    onUpdateIssue({
      ...issue,
      associatedRepoIds: next,
      updatedAt: new Date().toISOString(),
    });
  };

  const handleReworkSubmit = async () => {
    if (!reworkFeedback.trim()) {
      setChatError(lang === 'zh' ? '请输入评审修改意见' : 'Enter review feedback');
      return;
    }

    setShowReworkBox(false);
    setActiveTab('chat');
    const scope = reworkScope;
    setSelectedScope(scope);

    const targetLabel =
      scope === 'all'
        ? lang === 'zh'
          ? '整单需求'
          : 'whole requirement'
        : (issue.subRequirements || []).find((s) => s.id === scope)?.title || scope;

    const prompt =
      lang === 'zh'
        ? `开发者在评审中指出了以下问题，需要二次修改代码与开发文档（范围: ${targetLabel}）:\n"${reworkFeedback}"\n请重新分析并更新对应待开发文档。`
        : `Reviewer requested rework for ${targetLabel}:\n"${reworkFeedback}"\nPlease revise the Dev Spec(s) accordingly.`;

    const updatedIssue: Issue = {
      ...issue,
      reviewFeedback: reworkFeedback,
      reworkSubId: scope === 'all' ? '' : scope,
      autoDevLogs: [
        ...issue.autoDevLogs,
        {
          id: `rework-${Date.now()}`,
          timestamp: new Date().toLocaleTimeString(),
          phase: 'analyzing',
          message:
            lang === 'zh'
              ? `收到二次评审意见（${targetLabel}）: "${reworkFeedback}". 正在更新 Dev Spec...`
              : `Rework feedback (${targetLabel}): "${reworkFeedback}". Updating Dev Spec...`,
        },
      ],
    };

    onUpdateIssue(updatedIssue);
    const feedback = reworkFeedback;
    setReworkFeedback('');

    await handleSendMessage(prompt, { forceSpecSync: true, scope });
    onStartAutoDev(issue.id, scope === 'all' ? undefined : scope);
    void feedback;
  };

  const handleApproveMerge = async () => {
    try {
      const saved = await api.approveMerge(issue.id);
      onUpdateIssue(saved);
    } catch (err: any) {
      setChatError(err.message || t.mergeFailed);
    }
  };

  const handleExportDevSpec = async () => {
    const { filename, markdown } = specMarkdownForExport(
      issue,
      selectedScope,
      editingSpec ? specMarkdown : undefined
    );
    if (!markdown.trim()) {
      setChatError(t.noSpecToExport);
      setExportHint('');
      return;
    }
    setChatError('');
    setExportHint('');
    setExporting(true);
    try {
      const result = await saveTextFile(filename, markdown);
      if (result.status === 'cancelled') {
        return;
      }
      if (result.status === 'copied') {
        setExportHint(t.exportCopied);
        return;
      }
      setExportHint(
        result.path ? t.exportSavedTo.replace('{path}', result.path) : t.exportSaved
      );
    } catch (err: any) {
      setChatError(err?.message || t.exportFailed);
    } finally {
      setExporting(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-md p-4 sm:p-6">
      <div className={`w-full max-w-5xl border rounded-2xl shadow-2xl overflow-hidden flex flex-col h-[90vh] ${themeConfig.modalBg}`}>
        <IssueDetailHeader
          issue={issue}
          lang={lang}
          themeStyle={themeStyle}
          onClose={onClose}
          onStartAutoDev={onStartAutoDev}
          onDeleteIssue={onDeleteIssue}
          setActiveTab={setActiveTab}
        />

        <AssociatedReposBar
          issue={issue}
          gitRepos={gitRepos}
          associatedRepos={associatedRepos}
          canEditRepos={canEditRepos}
          editingRepos={editingRepos}
          setEditingRepos={setEditingRepos}
          toggleAssociatedRepo={toggleAssociatedRepo}
          lang={lang}
          themeStyle={themeStyle}
        />

        <StatusFocusBanner
          issue={issue}
          splitIssue={splitIssue}
          associatedRepos={associatedRepos}
          lang={lang}
          setActiveTab={setActiveTab}
          handleSendMessage={handleSendMessage}
          onUpdateIssue={onUpdateIssue}
          onStartAutoDev={onStartAutoDev}
          onCancelAutoDev={onCancelAutoDev}
          handleApproveMerge={handleApproveMerge}
        />

        <IssueDetailTabs
          issue={issue}
          splitIssue={splitIssue}
          activeTab={activeTab}
          setActiveTab={setActiveTab}
          lang={lang}
          themeStyle={themeStyle}
        />

        {/* Tab Content Body */}
        <div className="flex-1 overflow-hidden flex flex-col">
          {activeTab === 'chat' && (
            <IssueChatTab
              issue={issue}
              splitIssue={splitIssue}
              visibleMessages={visibleMessages}
              selectedScope={selectedScope}
              setSelectedScope={setSelectedScope}
              lang={lang}
              themeStyle={themeStyle}
              isSending={isSending}
              chatError={chatError}
              inputPrompt={inputPrompt}
              setInputPrompt={setInputPrompt}
              modelProcess={modelProcess}
              setModelProcess={setModelProcess}
              abortRef={abortRef}
              messagesEndRef={messagesEndRef}
              handleSendMessage={handleSendMessage}
            />
          )}

          {activeTab === 'spec' && (
            <IssueSpecTab
              issue={issue}
              splitIssue={splitIssue}
              activeSub={activeSub}
              currentSpec={currentSpec}
              selectedScope={selectedScope}
              setSelectedScope={setSelectedScope}
              lang={lang}
              themeStyle={themeStyle}
              editingSpec={editingSpec}
              setEditingSpec={setEditingSpec}
              specMarkdown={specMarkdown}
              setSpecMarkdown={setSpecMarkdown}
              exporting={exporting}
              exportHint={exportHint}
              chatError={chatError}
              setActiveTab={setActiveTab}
              handleSendMessage={handleSendMessage}
              handleExportDevSpec={handleExportDevSpec}
              onUpdateIssue={onUpdateIssue}
            />
          )}

          {activeTab === 'console' && (
            <IssueConsoleTab
              issue={issue}
              splitIssue={splitIssue}
              selectedScope={selectedScope}
              setSelectedScope={setSelectedScope}
              lang={lang}
              themeStyle={themeStyle}
              onStartAutoDev={onStartAutoDev}
            />
          )}

          {activeTab === 'review' && (
            <IssueReviewTab
              issue={issue}
              splitIssue={splitIssue}
              lang={lang}
              themeStyle={themeStyle}
              showReworkBox={showReworkBox}
              setShowReworkBox={setShowReworkBox}
              reworkFeedback={reworkFeedback}
              setReworkFeedback={setReworkFeedback}
              reworkScope={reworkScope}
              setReworkScope={setReworkScope}
              handleReworkSubmit={handleReworkSubmit}
              handleApproveMerge={handleApproveMerge}
            />
          )}
        </div>
      </div>
    </div>
  );
};
