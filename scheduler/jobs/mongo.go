/*
 * Copyright (c) 2019-2022. Abstrium SAS <team (at) pydio.com>
 * This file is part of Pydio Cells.
 *
 * Pydio Cells is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * Pydio Cells is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with Pydio Cells.  If not, see <http://www.gnu.org/licenses/>.
 *
 * The latest code can be found at <https://pydio.com>.
 */

package jobs

import (
	"context"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/pydio/cells/v4/common/dao/mongodb"
	"github.com/pydio/cells/v4/common/proto/jobs"
	"github.com/pydio/cells/v4/common/service/errors"
	"github.com/pydio/cells/v4/common/utils/configx"
)

const (
	collJobs  = "jobs"
	collTasks = "tasks"
)

var (
	model = &mongodb.Model{
		Collections: []mongodb.Collection{
			{
				Name: collJobs,
				Indexes: []map[string]int{
					{"id": 1},
					{"owner": 1, "has_events": 1, "has_schedule": 1},
				},
			},
			{
				Name: collTasks,
				Indexes: []map[string]int{
					{"id": 1, "job_id": 1, "status": 1, "ts": -1},
				},
			},
		},
	}
)

type mongoJob struct {
	ID          string `bson:"id"`
	Owner       string `bson:"owner"`
	HasEvents   bool   `bson:"has_events"`
	HasSchedule bool   `bson:"has_schedule"`
	*jobs.Job
}

type mongoTask struct {
	ID     string `bson:"id"`
	JobId  string `bson:"job_id"`
	Status int    `bson:"status"`
	Stamp  int64  `bson:"ts"`
	*jobs.Task
}

type mongoImpl struct {
	mongodb.DAO
}

func (m *mongoImpl) Init(values configx.Values) error {
	if er := model.Init(context.Background(), m.DB()); er != nil {
		return er
	}
	return m.DAO.Init(values)
}

func (m *mongoImpl) PutJob(job *jobs.Job) error {
	c := context.Background()
	// do not store tasks inside job
	mj := &mongoJob{
		ID:          job.ID,
		Owner:       job.Owner,
		HasEvents:   len(job.EventNames) > 0,
		HasSchedule: job.Schedule != nil,
		Job:         job,
	}
	mj.Job.Tasks = nil
	upsert := true
	_, e := m.DB().Collection(collJobs).ReplaceOne(c, bson.D{{"id", job.ID}}, mj, &options.ReplaceOptions{Upsert: &upsert})
	return e
}

func (m *mongoImpl) GetJob(jobId string, withTasks jobs.TaskStatus) (*jobs.Job, error) {
	c := context.Background()
	res := m.DB().Collection(collJobs).FindOne(c, bson.D{{"id", jobId}})
	if res.Err() != nil {
		if strings.Contains(res.Err().Error(), "no documents in result") {
			return nil, errors.NotFound("job.not.found", "Job not found")
		}
		return nil, res.Err()
	}
	mj := &mongoJob{}
	if er := res.Decode(&mj); er != nil {
		return nil, er
	}
	if withTasks != jobs.TaskStatus_Unknown {
		tt, e := m.listTasks(jobId, withTasks, 0, 0)
		if e != nil {
			return nil, e
		}
		mj.Job.Tasks = tt
	}
	return mj.Job, nil
}

func (m *mongoImpl) DeleteJob(jobId string) error {
	c := context.Background()
	_, e := m.DB().Collection(collJobs).DeleteOne(c, bson.D{{"id", jobId}})
	if e != nil {
		return e
	}
	//fmt.Println("Delete", res.DeletedCount, "job")
	return nil
}

func (m *mongoImpl) ListJobs(owner string, eventsOnly bool, timersOnly bool, withTasks jobs.TaskStatus, jobIDs []string, taskCursor ...int32) (chan *jobs.Job, chan bool, error) {
	c := context.Background()
	filter := bson.D{}
	if owner != "" {
		filter = append(filter, bson.E{Key: "owner", Value: owner})
	}
	if eventsOnly {
		filter = append(filter, bson.E{Key: "has_events", Value: true})
	} else if timersOnly {
		filter = append(filter, bson.E{Key: "has_schedule", Value: true})
	}
	if len(jobIDs) > 0 {
		filter = append(filter, bson.E{Key: "id", Value: bson.M{"$in": jobIDs}})
	}
	cursor, er := m.DB().Collection(collJobs).Find(c, filter)
	if er != nil {
		return nil, nil, er
	}
	cj := make(chan *jobs.Job)
	cd := make(chan bool, 1)

	var offset, limit int64
	if len(taskCursor) > 0 {
		offset = int64(taskCursor[0])
		if len(taskCursor) > 1 {
			limit = int64(taskCursor[1])
		}
	}

	go func() {
		defer close(cd)
		for cursor.Next(context.Background()) {
			mj := &mongoJob{}
			if er := cursor.Decode(&mj); er != nil {
				continue
			}
			if withTasks != jobs.TaskStatus_Unknown {
				tt, e := m.listTasks(mj.ID, withTasks, offset, limit)
				if e != nil {
					continue
				}
				if withTasks != jobs.TaskStatus_Any && len(tt) == 0 {
					continue
				}
				mj.Job.Tasks = tt
			}
			cj <- mj.Job
		}
	}()

	return cj, cd, nil

}

func (m *mongoImpl) PutTask(task *jobs.Task) error {
	c := context.Background()
	// do not store tasks inside job
	stripTaskData(task)
	mj := &mongoTask{
		ID:     task.ID,
		JobId:  task.JobID,
		Status: int(task.Status),
		Stamp:  int64(task.StartTime),
		Task:   task,
	}
	upsert := true
	_, e := m.DB().Collection(collTasks).ReplaceOne(c, bson.D{{"id", task.ID}}, mj, &options.ReplaceOptions{Upsert: &upsert})
	if e != nil {
		return e
	}
	//fmt.Println("Upserted task ", task.ID, res.UpsertedCount, res.ModifiedCount)
	return nil
}

func (m *mongoImpl) PutTasks(tasks map[string]map[string]*jobs.Task) error {
	var models []mongo.WriteModel
	for _, tt := range tasks {
		for _, t := range tt {
			mt := &mongoTask{
				ID:     t.ID,
				JobId:  t.JobID,
				Status: int(t.Status),
				Stamp:  int64(t.StartTime),
				Task:   t,
			}
			rModel := mongo.NewReplaceOneModel().
				SetFilter(bson.D{{"id", mt.ID}}).
				SetReplacement(mt).
				SetUpsert(true)
			models = append(models, rModel)
		}
	}
	_, e := m.DB().Collection(collTasks).BulkWrite(context.Background(), models)
	if e != nil {
		return e
	}
	//fmt.Println("Bulkwrite results modified", res.ModifiedCount, "inserted", res.UpsertedCount)
	return nil
}

func (m *mongoImpl) ListTasks(jobId string, taskStatus jobs.TaskStatus, cursor ...int32) (chan *jobs.Task, chan bool, error) {
	var offset, limit int64
	if len(cursor) > 0 {
		offset = int64(cursor[0])
		if len(cursor) > 1 {
			limit = int64(cursor[1])
		}
	}

	tt, e := m.listTasks(jobId, taskStatus, offset, limit)
	if e != nil {
		return nil, nil, e
	}
	cj := make(chan *jobs.Task)
	cd := make(chan bool, 1)
	go func() {
		defer close(cd)
		for _, t := range tt {
			cj <- t
		}
	}()
	return cj, cd, nil
}

func (m *mongoImpl) DeleteTasks(jobId string, taskId []string) error {
	filter := bson.D{{"job_id", jobId}, {"id", bson.M{"$in": taskId}}}
	_, e := m.DB().Collection(collTasks).DeleteMany(context.Background(), filter)
	if e != nil {
		return e
	}
	//fmt.Println("Deleted", res.DeletedCount, "tasks")
	return nil
}

func (m *mongoImpl) listTasks(jobId string, status jobs.TaskStatus, offset, limit int64) (tasks []*jobs.Task, e error) {
	filter := bson.D{}
	if jobId != "" {
		filter = append(filter, bson.E{"job_id", jobId})
	}
	if status != jobs.TaskStatus_Any {
		filter = append(filter, bson.E{"status", int(status)})
	}
	findOpts := &options.FindOptions{
		Sort: bson.M{"ts": -1},
	}
	if offset > 0 {
		findOpts.Skip = &offset
	}
	if limit > 0 {
		findOpts.Limit = &limit
	}
	c := context.Background()
	cursor, e := m.DB().Collection(collTasks).Find(c, filter, findOpts)
	if e != nil {
		return tasks, e
	}
	for cursor.Next(c) {
		mj := &mongoTask{}
		if er := cursor.Decode(mj); er != nil {
			continue
		}
		stripTaskData(mj.Task)
		tasks = append(tasks, mj.Task)
	}
	return
}
