import { API_HOST, DEFAULT_API_HOST } from '@/utils/const';

describe('API host configuration', () => {
  it('defaults to the Heima explorer API', () => {
    expect(DEFAULT_API_HOST).toBe('https://explorer-api.heima.network');
    expect(API_HOST).toBe(DEFAULT_API_HOST);
  });
});
