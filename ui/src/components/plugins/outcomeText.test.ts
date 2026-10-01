import { describe, expect, it } from 'vitest';
import { translate } from '../../i18n';
import { outcomeMessage, outcomeStatus } from './outcomeText';

const t = (key: string) => translate('zh-TW', key);

describe('plugin outcome translations', () => {
  it.each([
    ['installed', '已安裝'],
    ['unchanged', '沒有變更'],
    ['saved', '已儲存'],
    ['removed', '已移除'],
    ['excluded', '已排除'],
    ['imported', '已匯入'],
    ['failed', '失敗'],
  ])('localizes the %s status', (status, expected) => {
    expect(outcomeStatus(t, status)).toBe(expected);
  });

  it.each([
    ['Native installation recorded. Reload the Agent and complete any required login or hook trust.', '已記錄原生安裝。請重新載入 Agent，並完成必要的登入或 hooks 信任確認。'],
    ['Added to Skillshare. Choose Agents when you want to install it.', '已加入 Skillshare。想安裝時再選擇 Agent。'],
    ['Removed from Skillshare.', '已從 Skillshare 移除。'],
    ['Native plugin removed; shared marketplaces are retained.', '已移除原生 plugin；共用 marketplace 會保留。'],
    ['Sync selection saved. Run sync to apply installation changes.', '已儲存同步選擇。請執行同步以套用安裝變更。'],
    ['Removed from this target; plugin definition retained.', '已從這個工具移除；plugin 定義會保留。'],
    ['Existing installation adopted without changing its enabled state.', '已納入既有安裝，未變更其啟用狀態。'],
    ['Native operation completed, but recording its result failed; inspect status before retrying.', '原生操作已完成，但無法記錄結果；重試前請先檢查狀態。'],
  ])('localizes the fixed message: %s', (message, expected) => {
    expect(outcomeMessage(t, message)).toBe(expected);
  });

  it('preserves unknown statuses and native error details', () => {
    expect(outcomeStatus(t, 'new-status')).toBe('new-status');
    expect(outcomeMessage(t, 'Native authentication required')).toBe('Native authentication required');
    expect(outcomeMessage(t, '')).toBe('');
  });
});
