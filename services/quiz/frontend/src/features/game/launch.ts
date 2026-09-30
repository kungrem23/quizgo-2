import { ApiError, api } from '../../shared/api/client';
import type { CreatedGame } from '../../shared/api/types';
import { saveGameCredentials, type HostGameCredentials } from './credentials';

export class HostCredentialStorageError extends Error {}

interface LaunchDependencies {
  createGame: (quizId: number) => Promise<CreatedGame>;
  saveCredentials: (credentials: HostGameCredentials) => boolean;
}

const defaultDependencies: LaunchDependencies = {
  createGame: api.createGame,
  saveCredentials: saveGameCredentials,
};

export async function launchHostGame(
  quizId: number,
  dependencies: LaunchDependencies = defaultDependencies,
): Promise<CreatedGame> {
  const created = await dependencies.createGame(quizId);
  const saved = dependencies.saveCredentials({
    role: 'host',
    gameId: created.id,
    code: created.code,
    hostTicket: created.host_ticket,
    quizId,
  });
  if (!saved) {
    throw new HostCredentialStorageError('could not save host game credentials');
  }
  return created;
}

export function hostLaunchErrorMessage(error: unknown): string {
  if (error instanceof HostCredentialStorageError) {
    return 'Игра создана, но браузер не разрешил сохранить host ticket. Проверьте настройки хранилища перед повторной попыткой.';
  }
  if (!(error instanceof ApiError)) return 'Не удалось запустить игру. Попробуйте ещё раз.';
  if (error.status === 401) return 'Войдите в аккаунт, чтобы запустить игру.';
  if (error.status === 403) return 'Запустить игру может только автор квиза.';
  if (error.status === 404) return 'Квиз не найден. Возможно, он был удалён.';
  if (error.status === 409) {
    return 'Квиз пока не готов к игре. Проверьте вопросы и правильные ответы.';
  }
  if (error.status === 0) return error.message;
  return 'Game-service временно недоступен. Попробуйте позже.';
}
