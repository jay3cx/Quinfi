// API 通信模块
const API_BASE_URL = "http://localhost:8080/api/v1";

export interface FundInfo {
  code: string;
  name: string;
  type: string;
  manager: string;
  company: string;
  scale: number;
  nav: number;
  navDate: string;
  dayGrowth: number;
  weekGrowth: number;
  monthGrowth: number;
  threeMonthGrowth: number;
  sixMonthGrowth: number;
  yearGrowth: number;
}

export interface AnalysisResult {
  summary: string;
  performance: string;
  risk: string;
  holdings: string;
  recommendation: string;
}

export async function getFundInfo(code: string): Promise<FundInfo> {
  const response = await fetch(`${API_BASE_URL}/fund/${code}`);
  if (!response.ok) {
    throw new Error(`获取基金信息失败: ${response.statusText}`);
  }
  return response.json();
}

export async function analyzeFund(code: string): Promise<string> {
  const response = await fetch(`${API_BASE_URL}/chat`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({
      message: `分析基金 ${code}`,
      stream: false,
    }),
  });
  if (!response.ok) {
    throw new Error(`分析基金失败: ${response.statusText}`);
  }
  const data = await response.json();
  return data.response || data.content || "分析完成";
}
