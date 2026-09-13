// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { QuizImage } from './QuizImage';
import { api } from '../api/client';

const renderImage = (url?: string) =>
  render(
    <QueryClientProvider client={new QueryClient()}>
      <QuizImage imageId="image-1" url={url} alt="Вопрос" />
    </QueryClientProvider>,
  );
afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe('S3 image links', () => {
  it('uses the URL from the API directly', () => {
    const request = vi.spyOn(api, 'imageURL');
    renderImage('https://storage.test/bucket/images/image-1?signature=original');
    expect(screen.getByRole('img').getAttribute('src')).toContain('https://storage.test/');
    expect(request).not.toHaveBeenCalled();
  });
  it('refreshes an expired URL once and stops after another image error', async () => {
    const request = vi
      .spyOn(api, 'imageURL')
      .mockResolvedValue({ id: 'image-1', url: 'https://storage.test/fresh' });
    renderImage('https://storage.test/expired');
    fireEvent.error(screen.getByRole('img'));
    await waitFor(() =>
      expect(screen.getByRole('img').getAttribute('src')).toBe('https://storage.test/fresh'),
    );
    expect(request).toHaveBeenCalledTimes(1);
    fireEvent.error(screen.getByRole('img'));
    expect(screen.getByRole('status').textContent).toContain('Не удалось загрузить');
    expect(request).toHaveBeenCalledTimes(1);
  });
  it('resolves a restored draft that only contains the image ID', async () => {
    vi.spyOn(api, 'imageURL').mockResolvedValue({
      id: 'image-1',
      url: 'https://storage.test/restored',
    });
    renderImage();
    await waitFor(() =>
      expect(screen.getByRole('img').getAttribute('src')).toBe('https://storage.test/restored'),
    );
  });
});
