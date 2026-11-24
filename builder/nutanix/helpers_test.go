package nutanix

import (
	"strings"
	"testing"
	"time"

	imageModels "github.com/nutanix/ntnx-api-golang-clients/vmm-go-client/v4/models/vmm/v4/content"
)

func testImage(name, extID string, imageType imageModels.ImageType, sizeBytes int64, created time.Time) *imageModels.Image {
	img := imageModels.NewImage()
	n := name
	id := extID
	img.Name = &n
	img.ExtId = &id
	img.Type = imageType.Ref()
	sz := sizeBytes
	img.SizeBytes = &sz
	ct := created
	img.CreateTime = &ct
	return img
}

func TestSelectImageByNameAndType_PicksMatchingType(t *testing.T) {
	disk := testImage("rocky97", "disk-uuid", imageModels.IMAGETYPE_DISK_IMAGE, 100, time.Unix(1, 0))
	iso := testImage("rocky97", "iso-uuid", imageModels.IMAGETYPE_ISO_IMAGE, 100, time.Unix(2, 0))

	got, err := selectImageByNameAndType([]*imageModels.Image{disk, iso}, "rocky97", ImageTypeDiskImage, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ExtId == nil || *got.ExtId != "disk-uuid" {
		t.Fatalf("expected disk-uuid, got %v", got.ExtId)
	}

	got, err = selectImageByNameAndType([]*imageModels.Image{disk, iso}, "rocky97", ImageTypeISOImage, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ExtId == nil || *got.ExtId != "iso-uuid" {
		t.Fatalf("expected iso-uuid, got %v", got.ExtId)
	}
}

func TestSelectImageByNameAndType_WrongTypeError(t *testing.T) {
	iso := testImage("rocky97", "iso-uuid", imageModels.IMAGETYPE_ISO_IMAGE, 100, time.Unix(1, 0))

	_, err := selectImageByNameAndType([]*imageModels.Image{iso}, "rocky97", ImageTypeDiskImage, false)
	if err == nil {
		t.Fatal("expected type mismatch error")
	}
	if !strings.Contains(err.Error(), "DISK_IMAGE") || !strings.Contains(err.Error(), "ISO_IMAGE") {
		t.Fatalf("error should mention both types, got: %v", err)
	}
}

func TestSelectImageByNameAndType_SameTypeDuplicates(t *testing.T) {
	older := testImage("rocky97", "old-uuid", imageModels.IMAGETYPE_DISK_IMAGE, 100, time.Unix(1, 0))
	newer := testImage("rocky97", "new-uuid", imageModels.IMAGETYPE_DISK_IMAGE, 100, time.Unix(2, 0))

	_, err := selectImageByNameAndType([]*imageModels.Image{older, newer}, "rocky97", ImageTypeDiskImage, false)
	if err == nil {
		t.Fatal("expected duplicate error when allowDuplicates is false")
	}

	got, err := selectImageByNameAndType([]*imageModels.Image{older, newer}, "rocky97", ImageTypeDiskImage, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ExtId == nil || *got.ExtId != "new-uuid" {
		t.Fatalf("expected newest disk new-uuid, got %v", got.ExtId)
	}
}

func TestSelectImageByNameAndType_NameOnlyMixedTypesStillDuplicate(t *testing.T) {
	disk := testImage("rocky97", "disk-uuid", imageModels.IMAGETYPE_DISK_IMAGE, 100, time.Unix(1, 0))
	iso := testImage("rocky97", "iso-uuid", imageModels.IMAGETYPE_ISO_IMAGE, 100, time.Unix(2, 0))

	_, err := selectImageByNameAndType([]*imageModels.Image{disk, iso}, "rocky97", "", false)
	if err == nil {
		t.Fatal("expected duplicate error when type is omitted")
	}
}

func TestEnsureImageType(t *testing.T) {
	iso := testImage("rocky97", "iso-uuid", imageModels.IMAGETYPE_ISO_IMAGE, 100, time.Unix(1, 0))
	if err := ensureImageType(iso, ImageTypeISOImage, "iso-uuid"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err := ensureImageType(iso, ImageTypeDiskImage, "iso-uuid")
	if err == nil {
		t.Fatal("expected type mismatch")
	}
	if !strings.Contains(err.Error(), "ISO_IMAGE") || !strings.Contains(err.Error(), "DISK_IMAGE") {
		t.Fatalf("error should mention actual and expected types, got: %v", err)
	}
}
