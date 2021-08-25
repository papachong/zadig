package service

import (
	"go.uber.org/zap"

	systemmodel "github.com/koderover/zadig/pkg/microservice/aslan/core/system/repository/models"
	systemrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/system/repository/mongodb"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

func CreateAnnouncement(creater string, ctx *systemmodel.Announcement, log *zap.SugaredLogger) error {
	err := systemrepo.NewAnnouncementColl().Create(ctx)
	if err != nil {
		log.Errorf("create announcement failed, creater: %s, error: %s", creater, err)
		return e.ErrCreateNotify
	}

	return nil
}

func UpdateAnnouncement(user string, notifyID string, ctx *systemmodel.Announcement, log *zap.SugaredLogger) error {
	err := systemrepo.NewAnnouncementColl().Update(notifyID, ctx)
	if err != nil {
		log.Errorf("create announcement failed, user: %s, error: %s", user, err)
		return e.ErrUpdateNotify
	}

	return nil

}

func PullAllAnnouncement(user string, log *zap.SugaredLogger) ([]*systemmodel.Announcement, error) {
	resp, err := systemrepo.NewAnnouncementColl().List("*")
	if err != nil {
		log.Errorf("list announcement failed, user: %s, error: %s", user, err)
		return nil, e.ErrPullAllAnnouncement
	}

	return resp, nil
}

func PullNotifyAnnouncement(user string, log *zap.SugaredLogger) ([]*systemmodel.Announcement, error) {
	resp, err := systemrepo.NewAnnouncementColl().ListValidAnnouncements("*")
	if err != nil {
		log.Errorf("list announcement failed, user: %s, error: %s", user, err)
		return nil, e.ErrPullNotifyAnnouncement
	}

	return resp, nil
}

func DeleteAnnouncement(user, id string, log *zap.SugaredLogger) error {
	err := systemrepo.NewAnnouncementColl().DeleteAnnouncement(&systemrepo.AnnouncementDeleteArgs{ID: id})
	if err != nil {
		log.Errorf("Delete Announcement failed, user: %s, error: %s", user, err)
	}
	return err
}