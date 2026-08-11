import React, { useState } from 'react';
import { ModelConfig } from '../types';
import { testOpenAPIConnection } from '../lib/llm';
import { Language, ThemeStyle, getTranslation } from '../lib/i18n';
import { THEME_CONFIGS } from '../lib/theme';
import { X, Sparkles, CheckCircle2, AlertCircle, Loader2, Key, Globe, Cpu, Sliders } from 'lucide-react';

interface GlobalSettingsModalProps {
  isOpen: boolean;
  onClose: () => void;
  config: ModelConfig;
  onSave: (newConfig: ModelConfig) => void;
  themeStyle?: ThemeStyle;
  lang?: Language;
}

export const GlobalSettingsModal: React.FC<GlobalSettingsModalProps> = ({
  isOpen,
  onClose,
  config,
  onSave,
  themeStyle = 'glass',
  lang = 'zh',
}) => {
  const themeConfig = THEME_CONFIGS[themeStyle] || THEME_CONFIGS.glass;
  const isLight = themeConfig.isLight;
  const t = getTranslation(lang);

  const [openAIBaseUrl, setOpenAIBaseUrl] = useState(
    config.openAIBaseUrl || 'https://api.openai.com/v1/chat/completions'
  );
  const [openAIApiKey, setOpenAIApiKey] = useState('');
  const [openAIModel, setOpenAIModel] = useState(config.openAIModel || 'gpt-4o');
  const [temperature, setTemperature] = useState(config.temperature ?? 0.7);

  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState<{ success: boolean; message: string } | null>(null);

  if (!isOpen) return null;

  const handleTestConnection = async () => {
    setTesting(true);
    setTestResult(null);
    const res = await testOpenAPIConnection({
      openAIBaseUrl,
      openAIApiKey, // empty => server uses stored key
      openAIModel,
    });
    setTesting(false);
    if (res.success) {
      setTestResult({
        success: true,
        message: `连接成功！模型响应: "${res.message}"`,
      });
    } else {
      setTestResult({
        success: false,
        message: `连接失败: ${res.error}`,
      });
    }
  };

  const handleSave = () => {
    onSave({
      useCustomOpenAI: true,
      openAIBaseUrl: openAIBaseUrl.trim(),
      openAIApiKey: openAIApiKey.trim(), // empty keeps existing server-side key
      openAIModel: openAIModel.trim(),
      temperature,
    });
    onClose();
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-md p-4">
      <div className={`w-full max-w-2xl border rounded-2xl shadow-2xl overflow-hidden flex flex-col max-h-[90vh] ${themeConfig.modalBg}`}>
        {/* Modal Header */}
        <div className={`p-6 border-b flex items-center justify-between ${themeConfig.subtleBorder} ${themeConfig.modalHeaderBg}`}>
          <div className="flex items-center gap-3">
            <div className="p-2.5 rounded-xl bg-indigo-500/20 border border-indigo-500/30 text-indigo-500">
              <Cpu className="w-6 h-6" />
            </div>
            <div>
              <h2 className={`text-xl font-bold flex items-center gap-2 ${themeConfig.textPrimary}`}>
                大模型 (LLM) API 设置
              </h2>
              <p className={`text-xs mt-0.5 ${themeConfig.textSecondary}`}>
                配置 Vibecoding 调用的自定义 OpenAPI 端点 (支持 OpenAI, DeepSeek, Qwen 等)
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
        <div className="p-6 overflow-y-auto space-y-6 flex-1 text-sm">
          {/* Custom OpenAPI Form */}
          <div className="space-y-4 p-5 rounded-xl border border-indigo-500/30 bg-indigo-500/10">
            <div className="flex items-center gap-2 text-indigo-800 dark:text-indigo-300 font-medium pb-2 border-b border-indigo-500/20">
              <Globe className="w-4 h-4 text-indigo-500" />
              <span>OpenAPI 参数配置</span>
            </div>

            {/* Full chat completions URL */}
            <div>
              <label className={`block text-xs font-medium mb-1.5 ${themeConfig.textPrimary}`}>
                {t.baseUrlLabel}
              </label>
              <div className="relative">
                <input
                  type="text"
                  value={openAIBaseUrl}
                  onChange={(e) => setOpenAIBaseUrl(e.target.value)}
                  placeholder="https://api.openai.com/v1/chat/completions"
                  className={`w-full px-3.5 py-2.5 border rounded-xl focus:outline-none transition-colors font-mono text-xs ${themeConfig.inputBg} ${themeConfig.inputText} ${themeConfig.inputBorder}`}
                />
              </div>
              <p className={`text-[11px] mt-1 ${themeConfig.textMuted}`}>
                {t.baseUrlHint}
                <br />
                例: <code className="font-semibold">https://api.openai.com/v1/chat/completions</code>
                {' | '}
                <code className="font-semibold">https://api.deepseek.com/v1/chat/completions</code>
                {' | '}
                <code className="font-semibold">http://llm-gw.jd.local/v1/chat/completions</code>
              </p>
            </div>

            {/* API Key */}
            <div>
              <label className={`block text-xs font-medium mb-1.5 ${themeConfig.textPrimary}`}>
                API Key (密钥)
              </label>
              <div className="relative">
                <input
                  type="password"
                  value={openAIApiKey}
                  onChange={(e) => setOpenAIApiKey(e.target.value)}
                  placeholder={
                    config.keyConfigured
                      ? t.keyConfiguredKeep.replace('{hint}', config.keyHint || '****')
                      : t.keyPlaceholderServer
                  }
                  className={`w-full px-3.5 py-2.5 border rounded-xl focus:outline-none transition-colors font-mono text-xs pr-10 ${themeConfig.inputBg} ${themeConfig.inputText} ${themeConfig.inputBorder}`}
                />
                <Key className={`w-4 h-4 absolute right-3 top-3 ${themeConfig.textMuted}`} />
              </div>
              {config.keyConfigured && (
                <p className={`text-[11px] mt-1 text-emerald-600 dark:text-emerald-300`}>
                  {t.keyStoredHint.replace('{hint}', config.keyHint || '****')}
                </p>
              )}
            </div>

            {/* Model Name */}
            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className={`block text-xs font-medium mb-1.5 ${themeConfig.textPrimary}`}>
                  模型标识 (Model Name)
                </label>
                <input
                  type="text"
                  value={openAIModel}
                  onChange={(e) => setOpenAIModel(e.target.value)}
                  placeholder="gpt-4o / deepseek-r1 / deepseek-chat"
                  className={`w-full px-3.5 py-2.5 border rounded-xl focus:outline-none transition-colors font-mono text-xs ${themeConfig.inputBg} ${themeConfig.inputText} ${themeConfig.inputBorder}`}
                />
              </div>

              {/* Temperature */}
              <div>
                <label className={`block text-xs font-medium mb-1.5 flex items-center justify-between ${themeConfig.textPrimary}`}>
                  <span>采样温度 (Temperature)</span>
                  <span className="text-indigo-500 font-mono font-bold">{temperature}</span>
                </label>
                <input
                  type="range"
                  min="0"
                  max="1"
                  step="0.05"
                  value={temperature}
                  onChange={(e) => setTemperature(parseFloat(e.target.value))}
                  className="w-full h-2 rounded-lg appearance-none cursor-pointer accent-indigo-600 mt-2 bg-slate-200 dark:bg-black/40"
                />
              </div>
            </div>

            {/* Test Connection Button */}
            <div className="pt-2">
              <button
                type="button"
                onClick={handleTestConnection}
                disabled={testing || !openAIBaseUrl || (!openAIApiKey && !config.keyConfigured)}
                className="px-4 py-2 bg-indigo-500/20 hover:bg-indigo-500/30 border border-indigo-500/30 rounded-xl text-xs font-semibold text-indigo-800 dark:text-indigo-200 flex items-center gap-2 transition-colors disabled:opacity-50"
              >
                {testing ? (
                  <>
                    <Loader2 className="w-3.5 h-3.5 animate-spin text-indigo-500" />
                    测试 API 连接中...
                  </>
                ) : (
                  <>
                    <Sliders className="w-3.5 h-3.5 text-indigo-500" />
                    测试 OpenAPI 接口连通性
                  </>
                )}
              </button>

              {testResult && (
                <div
                  className={`mt-3 p-3 rounded-xl border text-xs flex items-start gap-2 ${
                    testResult.success
                      ? 'bg-emerald-500/20 border-emerald-500/30 text-emerald-800 dark:text-emerald-200'
                      : 'bg-rose-500/20 border-rose-500/30 text-rose-800 dark:text-rose-200'
                  }`}
                >
                  {testResult.success ? (
                    <CheckCircle2 className="w-4 h-4 text-emerald-500 shrink-0 mt-0.5" />
                  ) : (
                    <AlertCircle className="w-4 h-4 text-rose-500 shrink-0 mt-0.5" />
                  )}
                  <div className="break-all">{testResult.message}</div>
                </div>
              )}
            </div>
          </div>
        </div>

        {/* Modal Footer */}
        <div className={`p-4 border-t flex items-center justify-end gap-3 ${themeConfig.subtleBorder} ${themeConfig.modalHeaderBg}`}>
          <button
            onClick={onClose}
            className={`px-4 py-2 rounded-xl text-xs font-medium border transition-colors ${themeConfig.btnSecondary} ${themeConfig.btnSecondaryText}`}
          >
            取消
          </button>
          <button
            onClick={handleSave}
            className="px-5 py-2 rounded-xl text-xs font-semibold text-white bg-indigo-600 hover:bg-indigo-500 shadow-md transition-all"
          >
            保存配置
          </button>
        </div>
      </div>
    </div>
  );
};

