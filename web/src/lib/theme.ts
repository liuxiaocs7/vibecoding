import { ThemeStyle } from './i18n';

export interface ThemeConfig {
  id: ThemeStyle;
  nameEn: string;
  nameZh: string;
  isLight: boolean;
  appBg: string;
  sidebarBg: string;
  sidebarItemActive: string;
  sidebarItemHover: string;
  headerBg: string;
  cardBg: string;
  cardHoverBg: string;
  columnBg: string;
  columnHeaderBg: string;
  border: string;
  subtleBorder: string;
  textPrimary: string;
  textSecondary: string;
  textMuted: string;
  badgeRepoBg: string;
  badgeRepoText: string;
  badgeBranchBg: string;
  badgeBranchText: string;
  accentGlow: string;
  modalBg: string;
  modalHeaderBg: string;
  inputBg: string;
  inputText: string;
  inputBorder: string;
  btnSecondary: string;
  btnSecondaryHover: string;
  btnSecondaryText: string;
  aiChatBubbleBg: string;
  aiChatBubbleText: string;
  codeBlockBg: string;
}

export const THEME_CONFIGS: Record<ThemeStyle, ThemeConfig> = {
  glass: {
    id: 'glass',
    nameEn: 'Cyber Glass',
    nameZh: '赛博玻璃',
    isLight: false,
    appBg: 'bg-gradient-to-br from-slate-950 via-indigo-950 to-slate-900 text-slate-100',
    sidebarBg: 'bg-slate-900/60 backdrop-blur-xl border-white/10',
    sidebarItemActive: 'bg-indigo-600/30 border-indigo-400/40 text-white shadow-md font-semibold',
    sidebarItemHover: 'hover:bg-white/10 text-slate-300 hover:text-white',
    headerBg: 'bg-slate-900/60 backdrop-blur-xl border-white/10',
    cardBg: 'bg-slate-900/60 backdrop-blur-md border-white/15 hover:border-indigo-400/60 shadow-md',
    cardHoverBg: 'hover:bg-slate-800/80',
    columnBg: 'bg-slate-900/40 backdrop-blur-md border-white/10',
    columnHeaderBg: 'bg-slate-900/70 border-white/15 text-white',
    border: 'border-white/15',
    subtleBorder: 'border-white/10',
    textPrimary: 'text-white font-medium',
    textSecondary: 'text-slate-200',
    textMuted: 'text-slate-400',
    badgeRepoBg: 'bg-slate-950/80 border-indigo-500/30',
    badgeRepoText: 'text-indigo-200 font-semibold',
    badgeBranchBg: 'bg-indigo-950/80 border-indigo-500/40',
    badgeBranchText: 'text-indigo-200 font-semibold',
    accentGlow: 'from-indigo-500 to-purple-500',
    modalBg: 'bg-slate-950/95 backdrop-blur-2xl border-white/20 text-white',
    modalHeaderBg: 'bg-slate-900/80 border-white/15',
    inputBg: 'bg-slate-950/90',
    inputText: 'text-white placeholder-slate-400',
    inputBorder: 'border-white/20 focus:border-indigo-400',
    btnSecondary: 'bg-slate-800/80 border-slate-700 hover:bg-slate-700/80',
    btnSecondaryHover: 'hover:bg-slate-700/80',
    btnSecondaryText: 'text-white font-medium',
    aiChatBubbleBg: 'bg-slate-900/90 border-white/15',
    aiChatBubbleText: 'text-slate-100',
    codeBlockBg: 'bg-slate-950 border-slate-800',
  },
  slate: {
    id: 'slate',
    nameEn: 'Obsidian Slate',
    nameZh: '黑曜石',
    isLight: false,
    appBg: 'bg-slate-950 text-slate-100',
    sidebarBg: 'bg-slate-900 border-slate-800',
    sidebarItemActive: 'bg-slate-800 border-slate-700 text-slate-100 shadow-md font-semibold',
    sidebarItemHover: 'hover:bg-slate-800/50 text-slate-400 hover:text-slate-100',
    headerBg: 'bg-slate-900/90 border-slate-800',
    cardBg: 'bg-slate-800/80 border-slate-700/60 hover:border-indigo-400/50',
    cardHoverBg: 'hover:bg-slate-800',
    columnBg: 'bg-slate-900/60 border-slate-800/80',
    columnHeaderBg: 'bg-slate-800/80 border-slate-700 text-slate-100',
    border: 'border-slate-800',
    subtleBorder: 'border-slate-800/60',
    textPrimary: 'text-slate-100',
    textSecondary: 'text-slate-300',
    textMuted: 'text-slate-500',
    badgeRepoBg: 'bg-slate-900 border-slate-700',
    badgeRepoText: 'text-indigo-300',
    badgeBranchBg: 'bg-indigo-950/60 border-indigo-800/50',
    badgeBranchText: 'text-indigo-300',
    accentGlow: 'from-blue-600 to-indigo-600',
    modalBg: 'bg-slate-900 border-slate-700 text-slate-100',
    modalHeaderBg: 'bg-slate-800/50 border-slate-700',
    inputBg: 'bg-slate-950',
    inputText: 'text-slate-100 placeholder-slate-500',
    inputBorder: 'border-slate-700 focus:border-indigo-500',
    btnSecondary: 'bg-slate-800 border-slate-700 hover:bg-slate-700',
    btnSecondaryHover: 'hover:bg-slate-700',
    btnSecondaryText: 'text-slate-200',
    aiChatBubbleBg: 'bg-slate-800/90 border-slate-700',
    aiChatBubbleText: 'text-slate-200',
    codeBlockBg: 'bg-slate-950 border-slate-800',
  },
  oled: {
    id: 'oled',
    nameEn: 'Midnight OLED',
    nameZh: '纯黑极客',
    isLight: false,
    appBg: 'bg-black text-zinc-100',
    sidebarBg: 'bg-black border-zinc-800',
    sidebarItemActive: 'bg-zinc-900 border-zinc-700 text-zinc-100 shadow-md font-semibold',
    sidebarItemHover: 'hover:bg-zinc-900/60 text-zinc-400 hover:text-zinc-100',
    headerBg: 'bg-black border-zinc-800',
    cardBg: 'bg-zinc-900/90 border-zinc-800 hover:border-emerald-500/50',
    cardHoverBg: 'hover:bg-zinc-900',
    columnBg: 'bg-zinc-950 border-zinc-900',
    columnHeaderBg: 'bg-zinc-900 border-zinc-800 text-zinc-100',
    border: 'border-zinc-800',
    subtleBorder: 'border-zinc-900',
    textPrimary: 'text-zinc-100',
    textSecondary: 'text-zinc-300',
    textMuted: 'text-zinc-500',
    badgeRepoBg: 'bg-zinc-950 border-zinc-800',
    badgeRepoText: 'text-emerald-400',
    badgeBranchBg: 'bg-emerald-950/50 border-emerald-900/50',
    badgeBranchText: 'text-emerald-300',
    accentGlow: 'from-emerald-500 to-teal-500',
    modalBg: 'bg-zinc-950 border-zinc-800 text-zinc-100',
    modalHeaderBg: 'bg-zinc-900 border-zinc-800',
    inputBg: 'bg-black',
    inputText: 'text-zinc-100 placeholder-zinc-600',
    inputBorder: 'border-zinc-800 focus:border-emerald-500',
    btnSecondary: 'bg-zinc-900 border-zinc-800 hover:bg-zinc-800',
    btnSecondaryHover: 'hover:bg-zinc-800',
    btnSecondaryText: 'text-zinc-200',
    aiChatBubbleBg: 'bg-zinc-900 border-zinc-800',
    aiChatBubbleText: 'text-zinc-200',
    codeBlockBg: 'bg-zinc-950 border-zinc-900',
  },
  oat: {
    id: 'oat',
    nameEn: 'Warm Oats',
    nameZh: '暖阳燕麦',
    isLight: true,
    appBg: 'bg-[#F7F4EC] text-[#3A3228]',
    sidebarBg: 'bg-[#EFEAE0] border-[#E3DCCF] shadow-xs',
    sidebarItemActive: 'bg-[#E3DCCE] border-[#D1C7B5] text-[#2A2218] shadow-xs font-semibold',
    sidebarItemHover: 'hover:bg-[#E8E1D3] text-[#6E6153] hover:text-[#2A2218]',
    headerBg: 'bg-[#EFEAE0]/90 border-[#E3DCCF] shadow-xs backdrop-blur-md',
    cardBg: 'bg-[#FFFDF8] border-[#E3DCCF] hover:border-[#C26D38]/60 shadow-xs hover:shadow-sm',
    cardHoverBg: 'hover:bg-[#FAF6EE]',
    columnBg: 'bg-[#EBE4D6]/70 border-[#DDD5C5]/80',
    columnHeaderBg: 'bg-[#F3ECE0] border-[#E3DCCF] text-[#2A2218] shadow-2xs',
    border: 'border-[#E3DCCF]',
    subtleBorder: 'border-[#E8E1D3]',
    textPrimary: 'text-[#2A2218]',
    textSecondary: 'text-[#6E6153]',
    textMuted: 'text-[#A19484]',
    badgeRepoBg: 'bg-[#F2EBE0] border-[#DDD5C5]',
    badgeRepoText: 'text-[#C26D38] font-semibold',
    badgeBranchBg: 'bg-[#F8EFE2] border-[#EAD6BF]',
    badgeBranchText: 'text-[#B85D26] font-semibold',
    accentGlow: 'from-[#D97706] to-[#C26D38]',
    modalBg: 'bg-[#FAF6EE] border-[#E3DCCF] text-[#2A2218] shadow-2xl',
    modalHeaderBg: 'bg-[#F2EBE0] border-[#E3DCCF]',
    inputBg: 'bg-[#F3ECE0]',
    inputText: 'text-[#2A2218] placeholder-[#A19484]',
    inputBorder: 'border-[#DDD5C5] focus:border-[#C26D38] focus:bg-[#FFFDF8]',
    btnSecondary: 'bg-[#F2EBE0] border-[#E3DCCF] hover:bg-[#E8E1D3]',
    btnSecondaryHover: 'hover:bg-[#E8E1D3]',
    btnSecondaryText: 'text-[#3A3228]',
    aiChatBubbleBg: 'bg-[#F2EBE0] border-[#E3DCCF]',
    aiChatBubbleText: 'text-[#2A2218]',
    codeBlockBg: 'bg-[#2B241C] border-[#3D3429] text-[#F5F0E6]',
  },
  light: {
    id: 'light',
    nameEn: 'Clean Light',
    nameZh: '明亮极简',
    isLight: true,
    appBg: 'bg-slate-100 text-slate-900',
    sidebarBg: 'bg-white border-slate-200/90 shadow-sm',
    sidebarItemActive: 'bg-indigo-50 border-indigo-200 text-indigo-900 shadow-sm font-semibold',
    sidebarItemHover: 'hover:bg-slate-100 text-slate-600 hover:text-slate-900',
    headerBg: 'bg-white/90 border-slate-200/90 shadow-sm backdrop-blur-md',
    cardBg: 'bg-white border-slate-200 hover:border-indigo-400 shadow-sm hover:shadow-md',
    cardHoverBg: 'hover:bg-slate-50',
    columnBg: 'bg-slate-200/50 border-slate-200/80',
    columnHeaderBg: 'bg-white border-slate-200 text-slate-900 shadow-2xs',
    border: 'border-slate-200',
    subtleBorder: 'border-slate-200/60',
    textPrimary: 'text-slate-900',
    textSecondary: 'text-slate-600',
    textMuted: 'text-slate-400',
    badgeRepoBg: 'bg-slate-100 border-slate-300',
    badgeRepoText: 'text-indigo-700 font-semibold',
    badgeBranchBg: 'bg-indigo-50 border-indigo-200',
    badgeBranchText: 'text-indigo-800 font-semibold',
    accentGlow: 'from-indigo-600 to-blue-600',
    modalBg: 'bg-white border-slate-200 text-slate-900 shadow-2xl',
    modalHeaderBg: 'bg-slate-50 border-slate-200',
    inputBg: 'bg-slate-50',
    inputText: 'text-slate-900 placeholder-slate-400',
    inputBorder: 'border-slate-300 focus:border-indigo-500 focus:bg-white',
    btnSecondary: 'bg-slate-100 border-slate-200 hover:bg-slate-200',
    btnSecondaryHover: 'hover:bg-slate-200',
    btnSecondaryText: 'text-slate-800',
    aiChatBubbleBg: 'bg-slate-100 border-slate-200',
    aiChatBubbleText: 'text-slate-800',
    codeBlockBg: 'bg-slate-900 border-slate-800 text-slate-100',
  },
};

