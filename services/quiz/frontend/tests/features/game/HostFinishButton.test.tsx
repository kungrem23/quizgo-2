// @vitest-environment jsdom
import { act, cleanup, render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { HostFinishButton } from '../../../src/features/game/HostFinishButton';

afterEach(cleanup);

describe('host early finish', () => {
  it('requires confirmation and sends the finish command', async () => {
    let acknowledge!: (value: { duplicate: boolean }) => void;
    const acknowledged = new Promise<{ duplicate: boolean }>((resolve) => {
      acknowledge = resolve;
    });
    const sendCommand = vi.fn(() => ({ requestId: 'finish-1', acknowledged }));
    const user = userEvent.setup();

    render(<HostFinishButton sendCommand={sendCommand} />);
    await user.click(screen.getByRole('button', { name: 'Завершить игру' }));

    const dialog = screen.getByRole('dialog');
    expect(within(dialog).getByRole('heading', { name: 'Завершить игру досрочно?' })).toBeTruthy();
    expect(sendCommand).not.toHaveBeenCalled();

    const confirm = within(dialog).getByRole('button', { name: 'Завершить игру' });
    await user.click(confirm);
    expect(sendCommand).toHaveBeenCalledWith({ type: 'finish' });
    expect((confirm as HTMLButtonElement).disabled).toBe(true);

    await act(async () => acknowledge({ duplicate: false }));
    expect(screen.queryByRole('dialog')).toBeNull();
  });
});
