import { colors } from '../../design';

export function riskColor(risk: string): string {
  switch (risk) {
    case 'critical': return colors.danger;
    case 'high': return colors.warning;
    case 'medium': return colors.blue;
    case 'low': return colors.success;
    default: return colors.success;
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
