package asset

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

// mockDBStore implements DBStore for unit tests.
type mockDBStore struct {
	createAssetFn func(ctx context.Context, arg refdata.CreateAssetParams) (refdata.Asset, error)
	getAssetFn    func(ctx context.Context, id uuid.UUID) (refdata.Asset, error)
	deleteAssetFn func(ctx context.Context, id uuid.UUID) error
	listAssetsFn  func(ctx context.Context, campaignID uuid.UUID) ([]refdata.Asset, error)
}

func (m *mockDBStore) CreateAsset(ctx context.Context, arg refdata.CreateAssetParams) (refdata.Asset, error) {
	if m.createAssetFn != nil {
		return m.createAssetFn(ctx, arg)
	}
	return refdata.Asset{}, nil
}

func (m *mockDBStore) GetAssetByID(ctx context.Context, id uuid.UUID) (refdata.Asset, error) {
	if m.getAssetFn != nil {
		return m.getAssetFn(ctx, id)
	}
	return refdata.Asset{}, sql.ErrNoRows
}

func (m *mockDBStore) DeleteAsset(ctx context.Context, id uuid.UUID) error {
	if m.deleteAssetFn != nil {
		return m.deleteAssetFn(ctx, id)
	}
	return nil
}

func (m *mockDBStore) ListAssetsByCampaignID(ctx context.Context, campaignID uuid.UUID) ([]refdata.Asset, error) {
	if m.listAssetsFn != nil {
		return m.listAssetsFn(ctx, campaignID)
	}
	return nil, nil
}

// mockFileStore implements Store for unit tests.
type mockFileStore struct {
	putFn    func(ctx context.Context, campaignID uuid.UUID, assetType AssetType, filename string, r io.Reader) (string, error)
	getFn    func(ctx context.Context, storagePath string) (io.ReadCloser, error)
	deleteFn func(ctx context.Context, storagePath string) error
	urlFn    func(assetID uuid.UUID) string
}

func (m *mockFileStore) Put(ctx context.Context, campaignID uuid.UUID, assetType AssetType, filename string, r io.Reader) (string, error) {
	if m.putFn != nil {
		return m.putFn(ctx, campaignID, assetType, filename, r)
	}
	return "mock/path", nil
}

func (m *mockFileStore) Get(ctx context.Context, storagePath string) (io.ReadCloser, error) {
	if m.getFn != nil {
		return m.getFn(ctx, storagePath)
	}
	return io.NopCloser(strings.NewReader("mock")), nil
}

func (m *mockFileStore) Delete(ctx context.Context, storagePath string) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, storagePath)
	}
	return nil
}

func (m *mockFileStore) URL(assetID uuid.UUID) string {
	if m.urlFn != nil {
		return m.urlFn(assetID)
	}
	return "/api/assets/" + assetID.String()
}

func TestService_Upload(t *testing.T) {
	campaignID := uuid.New()
	assetID := uuid.New()

	db := &mockDBStore{
		createAssetFn: func(ctx context.Context, arg refdata.CreateAssetParams) (refdata.Asset, error) {
			return refdata.Asset{
				ID:           assetID,
				CampaignID:   arg.CampaignID,
				Type:         arg.Type,
				OriginalName: arg.OriginalName,
				MimeType:     arg.MimeType,
				ByteSize:     arg.ByteSize,
				StoragePath:  arg.StoragePath,
			}, nil
		},
	}

	fs := &mockFileStore{
		putFn: func(ctx context.Context, cid uuid.UUID, at AssetType, fn string, r io.Reader) (string, error) {
			return "camp/maps/abc", nil
		},
	}

	svc := NewService(db, fs)

	asset, err := svc.Upload(context.Background(), UploadInput{
		CampaignID:   campaignID,
		Type:         TypeMapBackground,
		OriginalName: "bg.png",
		MimeType:     "image/png",
		Content:      bytes.NewReader([]byte("imagedata")),
	})
	require.NoError(t, err)
	assert.Equal(t, assetID, asset.ID)
	assert.Equal(t, "image/png", asset.MimeType)
}

func TestService_Upload_InvalidType(t *testing.T) {
	svc := NewService(&mockDBStore{}, &mockFileStore{})

	_, err := svc.Upload(context.Background(), UploadInput{
		CampaignID:   uuid.New(),
		Type:         AssetType("bogus"),
		OriginalName: "f.png",
		MimeType:     "image/png",
		Content:      bytes.NewReader([]byte("x")),
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid asset type")
}

func TestService_Upload_EmptyName(t *testing.T) {
	svc := NewService(&mockDBStore{}, &mockFileStore{})

	_, err := svc.Upload(context.Background(), UploadInput{
		CampaignID:   uuid.New(),
		Type:         TypeToken,
		OriginalName: "",
		MimeType:     "image/png",
		Content:      bytes.NewReader([]byte("x")),
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "original_name")
}

func TestService_Upload_StoreFails(t *testing.T) {
	fs := &mockFileStore{
		putFn: func(ctx context.Context, cid uuid.UUID, at AssetType, fn string, r io.Reader) (string, error) {
			return "", errors.New("disk full")
		},
	}
	svc := NewService(&mockDBStore{}, fs)

	_, err := svc.Upload(context.Background(), UploadInput{
		CampaignID:   uuid.New(),
		Type:         TypeToken,
		OriginalName: "f.png",
		MimeType:     "image/png",
		Content:      bytes.NewReader([]byte("x")),
	})
	assert.Error(t, err)
}

func TestService_GetByID(t *testing.T) {
	assetID := uuid.New()
	db := &mockDBStore{
		getAssetFn: func(ctx context.Context, id uuid.UUID) (refdata.Asset, error) {
			return refdata.Asset{ID: assetID, StoragePath: "a/b/c"}, nil
		},
	}
	svc := NewService(db, &mockFileStore{})

	a, err := svc.GetByID(context.Background(), assetID)
	require.NoError(t, err)
	assert.Equal(t, assetID, a.ID)
}

func TestService_GetByID_NotFound(t *testing.T) {
	db := &mockDBStore{
		getAssetFn: func(ctx context.Context, id uuid.UUID) (refdata.Asset, error) {
			return refdata.Asset{}, sql.ErrNoRows
		},
	}
	svc := NewService(db, &mockFileStore{})

	_, err := svc.GetByID(context.Background(), uuid.New())
	assert.Error(t, err)
}

func TestService_Delete(t *testing.T) {
	assetID := uuid.New()
	deletedFromFS := false
	deletedFromDB := false

	db := &mockDBStore{
		getAssetFn: func(ctx context.Context, id uuid.UUID) (refdata.Asset, error) {
			return refdata.Asset{ID: assetID, StoragePath: "a/b/c"}, nil
		},
		deleteAssetFn: func(ctx context.Context, id uuid.UUID) error {
			deletedFromDB = true
			return nil
		},
	}
	fs := &mockFileStore{
		deleteFn: func(ctx context.Context, sp string) error {
			deletedFromFS = true
			return nil
		},
	}

	svc := NewService(db, fs)
	err := svc.Delete(context.Background(), assetID)
	require.NoError(t, err)
	assert.True(t, deletedFromDB)
	assert.True(t, deletedFromFS)
}

func TestService_Upload_DBFails(t *testing.T) {
	fileDeleteCalled := false
	db := &mockDBStore{
		createAssetFn: func(ctx context.Context, arg refdata.CreateAssetParams) (refdata.Asset, error) {
			return refdata.Asset{}, errors.New("db error")
		},
	}
	fs := &mockFileStore{
		putFn: func(ctx context.Context, cid uuid.UUID, at AssetType, fn string, r io.Reader) (string, error) {
			return "some/path", nil
		},
		deleteFn: func(ctx context.Context, sp string) error {
			fileDeleteCalled = true
			return nil
		},
	}
	svc := NewService(db, fs)

	_, err := svc.Upload(context.Background(), UploadInput{
		CampaignID:   uuid.New(),
		Type:         TypeToken,
		OriginalName: "f.png",
		MimeType:     "image/png",
		Content:      bytes.NewReader([]byte("x")),
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "creating asset record")
	assert.True(t, fileDeleteCalled, "should cleanup stored file on DB failure")
}

func TestService_Upload_EmptyMimeType(t *testing.T) {
	svc := NewService(&mockDBStore{}, &mockFileStore{})

	_, err := svc.Upload(context.Background(), UploadInput{
		CampaignID:   uuid.New(),
		Type:         TypeToken,
		OriginalName: "f.png",
		MimeType:     "",
		Content:      bytes.NewReader([]byte("x")),
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "mime_type")
}

func TestService_Upload_NilContent(t *testing.T) {
	svc := NewService(&mockDBStore{}, &mockFileStore{})

	_, err := svc.Upload(context.Background(), UploadInput{
		CampaignID:   uuid.New(),
		Type:         TypeToken,
		OriginalName: "f.png",
		MimeType:     "image/png",
		Content:      nil,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "content")
}

func TestService_Delete_GetFails(t *testing.T) {
	db := &mockDBStore{
		getAssetFn: func(ctx context.Context, id uuid.UUID) (refdata.Asset, error) {
			return refdata.Asset{}, errors.New("not found")
		},
	}
	svc := NewService(db, &mockFileStore{})
	err := svc.Delete(context.Background(), uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "getting asset")
}

func TestService_Delete_FileDeleteFails(t *testing.T) {
	db := &mockDBStore{
		getAssetFn: func(ctx context.Context, id uuid.UUID) (refdata.Asset, error) {
			return refdata.Asset{StoragePath: "a/b/c"}, nil
		},
	}
	fs := &mockFileStore{
		deleteFn: func(ctx context.Context, sp string) error {
			return errors.New("permission denied")
		},
	}
	svc := NewService(db, fs)
	err := svc.Delete(context.Background(), uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "deleting file")
}

func TestService_Delete_DBDeleteFails(t *testing.T) {
	db := &mockDBStore{
		getAssetFn: func(ctx context.Context, id uuid.UUID) (refdata.Asset, error) {
			return refdata.Asset{StoragePath: "a/b/c"}, nil
		},
		deleteAssetFn: func(ctx context.Context, id uuid.UUID) error {
			return errors.New("db error")
		},
	}
	svc := NewService(db, &mockFileStore{})
	err := svc.Delete(context.Background(), uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "deleting asset record")
}

func TestService_URL(t *testing.T) {
	id := uuid.New()
	fs := &mockFileStore{
		urlFn: func(assetID uuid.UUID) string {
			return "/api/assets/" + assetID.String()
		},
	}
	svc := NewService(&mockDBStore{}, fs)
	assert.Equal(t, "/api/assets/"+id.String(), svc.URL(id))
}

func TestService_OpenFile(t *testing.T) {
	assetID := uuid.New()
	db := &mockDBStore{
		getAssetFn: func(ctx context.Context, id uuid.UUID) (refdata.Asset, error) {
			return refdata.Asset{ID: assetID, StoragePath: "a/b/c", MimeType: "image/png"}, nil
		},
	}
	fs := &mockFileStore{
		getFn: func(ctx context.Context, sp string) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("filedata")), nil
		},
	}

	svc := NewService(db, fs)
	a, rc, err := svc.OpenFile(context.Background(), assetID)
	require.NoError(t, err)
	defer rc.Close()

	assert.Equal(t, "image/png", a.MimeType)
	data, _ := io.ReadAll(rc)
	assert.Equal(t, "filedata", string(data))
}
