/** Base path injected by the Go server for reverse proxy deployments. */
export const BASE_PATH: string = (window as any).__BASE_PATH__ || '';
