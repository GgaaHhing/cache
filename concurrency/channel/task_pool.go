package channel

import "context"

type Task func()

type TaskPool struct {
	tasks chan Task
	close chan struct{}
	// close *atomic.Bool

	// 为了防止重复调用close
	//closeOnce sync.Once
}

func NewTaskPool(numG int, capacity int) *TaskPool {
	res := &TaskPool{
		tasks: make(chan Task, capacity),
		close: make(chan struct{}),
	}
	for i := 0; i < numG; i++ {
		go func() {
			for {
				select {
				case <-res.close:
					return
				case task := <-res.tasks:
					task()
				}
			}
		}()
	}
	return res
}

// Submit 提交任务
func (p *TaskPool) Submit(ctx context.Context, t Task) error {

	select {
	// 把t放进去
	case p.tasks <- t:
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

// Close
// 重复调用close会导致panic, 不要重复调用
func (p *TaskPool) Close() error {
	//p.close.Store(true)
	//p.closeOnce.Do(func() {
	//	close(p.close)
	//})
	close(p.close)
	return nil
}
