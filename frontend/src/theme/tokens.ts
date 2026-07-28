import { darkTheme } from "naive-ui";
import type { GlobalThemeOverrides } from "naive-ui";

export const naiveTheme = darkTheme;

export const themeOverrides: GlobalThemeOverrides = {
  common: {
    primaryColor: "#0EA5E9",
    primaryColorHover: "#38bdf8",
    primaryColorPressed: "#0284c7",
    primaryColorSuppl: "#0284c7",
    bodyColor: "#0f1828",
    cardColor: "rgba(255,255,255,0.05)",
    modalColor: "#1a2440",
    popoverColor: "#1a2440",
    borderColor: "rgba(255,255,255,0.12)",
    borderRadius: "12px",
    fontSize: "13px",
  },
  Card: { borderRadius: "14px", paddingLarge: "20px" },
  Button: { borderRadiusMedium: "10px", fontWeightStrong: "600" },
};

// 品牌色（与 logo / 状态环一致）
export const BRAND = "#0EA5E9";
export const GOOD = "#22c55e";
export const WARN = "#fbbf24";
export const DANGER = "#f87171";
