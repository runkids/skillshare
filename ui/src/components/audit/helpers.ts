import { palette } from '../../design';

export function riskColor(risk: string): string {
  switch (risk) {
    case 'critical': return palette.danger;
    case 'high': return palette.warning;
    case 'medium': return palette.info;
    case 'low': return palette.success;
    default: return palette.success;
  }
}

export function riskBgColor(risk: string): string {
  switch (risk) {
    case 'critical': return 'rgba(192, 57, 43, 0.06)';
    case 'high': return 'rgba(212, 135, 14, 0.06)';
    case 'medium': return 'rgba(45, 93, 161, 0.06)';
    case 'low': return 'rgba(46, 139, 87, 0.06)';
    default: return 'rgba(46, 139, 87, 0.06)';
  }
}
