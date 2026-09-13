package quiz

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
)

var ErrImageStorage = errors.New("image storage unavailable")

type ImageStore interface {
	Put(context.Context, string, string, []byte) error
	URL(context.Context, string) (string, error)
	Delete(context.Context, string) error
}

func storageError(err error) error { return fmt.Errorf("%w: %v", ErrImageStorage, err) }

func (s *Service) resolveImages(ctx context.Context, questions []ContentQuestion) error {
	urls := map[string]string{}
	for i := range questions {
		q := &questions[i]
		q.ImageURL = ""
		if q.ImageID == "" {
			continue
		}
		if s.images == nil {
			return ErrImageStorage
		}
		if _, ok := urls[q.ImageID]; !ok {
			link, err := s.images.URL(ctx, q.ImageID)
			if err != nil {
				return storageError(err)
			}
			urls[q.ImageID] = link
		}
		q.ImageURL = urls[q.ImageID]
	}
	return nil
}

func (s *Service) UploadImage(ctx context.Context, userID int, contentType string, content []byte) (UploadedImage, error) {
	if s.images == nil {
		return UploadedImage{}, ErrImageStorage
	}
	v := UploadedImage{ID: uuid.NewString()}
	var err error
	v.URL, err = s.images.URL(ctx, v.ID)
	if err != nil {
		return UploadedImage{}, storageError(err)
	}
	if err = s.images.Put(ctx, v.ID, contentType, content); err != nil {
		return UploadedImage{}, storageError(err)
	}
	if err = s.repo.CreateImageRecord(ctx, v.ID, userID); err != nil {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		if cleanupErr := s.images.Delete(cleanupCtx, v.ID); cleanupErr != nil {
			log.Printf("cleanup orphan image %s: %v", v.ID, cleanupErr)
		}
		return UploadedImage{}, err
	}
	return v, nil
}

func (s *Service) ReadImage(ctx context.Context, id string) (UploadedImage, error) {
	if err := s.repo.ImageExists(ctx, id); err != nil {
		return UploadedImage{}, err
	}
	if s.images == nil {
		return UploadedImage{}, ErrImageStorage
	}
	link, err := s.images.URL(ctx, id)
	if err != nil {
		return UploadedImage{}, storageError(err)
	}
	return UploadedImage{ID: id, URL: link}, nil
}

func (s *Service) DeleteUploadedImage(ctx context.Context, id string, userID int) error {
	if s.images == nil {
		return ErrImageStorage
	}
	return s.repo.DeleteUploadedImage(ctx, id, userID, func(ctx context.Context, id string) error {
		if err := s.images.Delete(ctx, id); err != nil {
			return storageError(err)
		}
		return nil
	})
}
