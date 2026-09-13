package quiz

import (
	"context"
	"errors"
	"testing"
)

type imageRepoStub struct {
	Repository
	create func(context.Context, string, int) error
}

func (r imageRepoStub) CreateImageRecord(ctx context.Context, id string, owner int) error {
	return r.create(ctx, id, owner)
}

type imageStoreStub struct {
	putErr            error
	urlErr            error
	uploaded, deleted string
	cleanupCanceled   bool
}

func (s *imageStoreStub) Put(_ context.Context, id, _ string, _ []byte) error {
	s.uploaded = id
	return s.putErr
}
func (s *imageStoreStub) URL(_ context.Context, id string) (string, error) {
	return "https://s3.example/images/" + id, s.urlErr
}
func (s *imageStoreStub) Delete(ctx context.Context, id string) error {
	s.deleted = id
	s.cleanupCanceled = ctx.Err() != nil
	return nil
}

func TestUploadCompensatesForDBFailureEvenAfterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dbErr := errors.New("insert failed")
	store := &imageStoreStub{}
	service := NewService(imageRepoStub{create: func(_ context.Context, id string, owner int) error {
		if id != store.uploaded || owner != 42 {
			t.Fatal("wrong metadata")
		}
		cancel()
		return dbErr
	}}, store)
	_, err := service.UploadImage(ctx, 42, "image/png", []byte("image"))
	if !errors.Is(err, dbErr) || store.uploaded == "" || store.deleted != store.uploaded || store.cleanupCanceled {
		t.Fatalf("err=%v store=%+v", err, store)
	}
}

func TestFailedS3UploadDoesNotCreateMetadata(t *testing.T) {
	store := &imageStoreStub{putErr: errors.New("offline")}
	service := NewService(imageRepoStub{create: func(context.Context, string, int) error { t.Fatal("metadata was inserted"); return nil }}, store)
	if _, err := service.UploadImage(context.Background(), 42, "image/png", nil); !errors.Is(err, ErrImageStorage) {
		t.Fatal(err)
	}
}

func TestImageLinksReplaceClientURLsAndDoNotPersistInRepository(t *testing.T) {
	service := NewService(nil, &imageStoreStub{})
	q := []ContentQuestion{{ImageID: "image-1", ImageURL: "https://untrusted.invalid"}, {ImageURL: "https://untrusted.invalid"}}
	if err := service.resolveImages(context.Background(), q); err != nil {
		t.Fatal(err)
	}
	if q[0].ImageURL != "https://s3.example/images/image-1" || q[1].ImageURL != "" {
		t.Fatal(q)
	}
	if _, err := NewService(nil).UploadImage(context.Background(), 42, "image/png", nil); !errors.Is(err, ErrImageStorage) {
		t.Fatal(err)
	}
}
